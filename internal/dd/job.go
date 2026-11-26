package dd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ddeck/internal/disk"
)

// Status est l'état d'une copie.
type Status int

const (
	Running Status = iota
	Done
	Failed
	Canceled
)

// ProgressMsg est envoyé à chaque ligne de progression de dd.
type ProgressMsg struct {
	ID      int
	Bytes   int64
	Elapsed float64
}

// DoneMsg est envoyé quand dd se termine.
type DoneMsg struct {
	ID  int
	Err error
}

// Job est une copie dd. Ses champs d'état ne sont modifiés que par la boucle de l'UI
// (via Apply/Finish), le processus ne communique que par messages.
type Job struct {
	ID   int
	Src  disk.Disk
	Dst  disk.Disk
	Plan Plan

	Status   Status
	Bytes    int64
	Total    int64
	Speed    float64 // octets/s, lissée
	Started  time.Time
	Ended    time.Time
	Err      string
	canceled bool

	lastBytes   int64
	lastElapsed float64
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewJob prépare une copie.
func NewJob(id int, src, dst disk.Disk, plan Plan) *Job {
	return &Job{ID: id, Src: src, Dst: dst, Plan: plan, Total: src.Size, done: make(chan struct{})}
}

var progressRe = regexp.MustCompile(`^(\d+) bytes .*copied, ([\d.]+) s`)

// ParseProgress extrait octets copiés et secondes écoulées d'une ligne de dd.
func ParseProgress(line string) (int64, float64, bool) {
	m := progressRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return 0, 0, false
	}
	b, err1 := strconv.ParseInt(m[1], 10, 64)
	s, err2 := strconv.ParseFloat(m[2], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return b, s, true
}

// scanCRLF découpe sur \r et \n (dd réécrit sa ligne de progression avec \r).
func scanCRLF(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// Start lance dd ; send reçoit ProgressMsg puis un DoneMsg final.
func (j *Job) Start(send func(any)) error {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "dd", j.Plan.Args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	j.cancel = cancel
	j.Started = time.Now()
	j.Status = Running

	go func() {
		defer close(j.done)
		defer cancel()
		var tail []string
		sc := bufio.NewScanner(stderr)
		sc.Split(scanCRLF)
		for sc.Scan() {
			line := sc.Text()
			if b, s, ok := ParseProgress(line); ok {
				send(ProgressMsg{ID: j.ID, Bytes: b, Elapsed: s})
				continue
			}
			if l := strings.TrimSpace(line); l != "" && !strings.Contains(l, "records") {
				tail = append(tail, l)
				if len(tail) > 3 {
					tail = tail[1:]
				}
			}
		}
		err := cmd.Wait()
		if err != nil && len(tail) > 0 {
			err = fmt.Errorf("%s", strings.Join(tail, " | "))
		}
		send(DoneMsg{ID: j.ID, Err: err})
	}()
	return nil
}

// Apply met à jour la progression (appelé depuis l'UI).
func (j *Job) Apply(m ProgressMsg) {
	if m.Elapsed > j.lastElapsed && m.Bytes >= j.lastBytes {
		inst := float64(m.Bytes-j.lastBytes) / (m.Elapsed - j.lastElapsed)
		if j.Speed == 0 {
			j.Speed = inst
		} else {
			j.Speed = 0.3*inst + 0.7*j.Speed
		}
	}
	j.Bytes, j.lastBytes, j.lastElapsed = m.Bytes, m.Bytes, m.Elapsed
}

// Finish enregistre la fin de dd (appelé depuis l'UI).
func (j *Job) Finish(m DoneMsg) {
	j.Ended = time.Now()
	switch {
	case j.canceled:
		j.Status = Canceled
	case m.Err != nil:
		j.Status = Failed
		j.Err = m.Err.Error()
	default:
		j.Status = Done
		if j.Bytes < j.Total {
			j.Bytes = j.Total
		}
	}
}

// Fail marque une copie qui n'a pas pu démarrer.
func (j *Job) Fail(err error) {
	j.Status, j.Err, j.Started, j.Ended = Failed, err.Error(), time.Now(), time.Now()
	close(j.done)
}

// Cancel interrompt dd (SIGINT).
func (j *Job) Cancel() {
	if j.Status == Running && j.cancel != nil {
		j.canceled = true
		j.cancel()
	}
}

// Wait attend la fin du processus, au plus d.
func (j *Job) Wait(d time.Duration) {
	select {
	case <-j.done:
	case <-time.After(d):
	}
}

// Percent renvoie l'avancement entre 0 et 1.
func (j *Job) Percent() float64 {
	if j.Total <= 0 {
		return 0
	}
	p := float64(j.Bytes) / float64(j.Total)
	if p > 1 {
		p = 1
	}
	return p
}

// ETA estime le temps restant (0 si inconnu).
func (j *Job) ETA() time.Duration {
	if j.Speed <= 0 || j.Bytes >= j.Total {
		return 0
	}
	return time.Duration(float64(j.Total-j.Bytes) / j.Speed * float64(time.Second))
}

// Elapsed renvoie la durée de la copie.
func (j *Job) Elapsed() time.Duration {
	if j.Started.IsZero() {
		return 0
	}
	if !j.Ended.IsZero() {
		return j.Ended.Sub(j.Started)
	}
	return time.Since(j.Started)
}

// Syncing est vrai quand tout est copié mais que dd vide encore les tampons (conv=fsync).
func (j *Job) Syncing() bool { return j.Status == Running && j.Bytes >= j.Total }

// Running est vrai tant que dd tourne.
func (j *Job) Running() bool { return j.Status == Running }
