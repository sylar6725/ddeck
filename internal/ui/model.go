// Package ui implements ddeck's Bubble Tea interface.
package ui

import (
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ddeck/internal/dd"
	"ddeck/internal/disk"
	"ddeck/internal/i18n"
)

// tr returns en or fr depending on the active language.
func tr(en, fr string) string { return i18n.S(en, fr) }

type state int

const (
	stSource state = iota
	stDest
	stConfirm
	stJobs
	stQuit
)

type scanMsg struct {
	disks   []disk.Disk
	content map[string]disk.Content
	err     error
}

type tickMsg time.Time

// Model is the application's root model.
type Model struct {
	isRoot bool
	send   func(tea.Msg)

	width, height int

	disks    []disk.Disk
	content  map[string]disk.Content
	scanning bool
	scanErr  error

	state, prev    state
	srcCur, dstCur int
	src, dst       *disk.Disk

	confirm confirmState

	jobs   []*dd.Job
	jobCur int
	nextID int

	bar    progress.Model
	status string
}

// New creates the model.
func New(isRoot bool) *Model {
	return &Model{
		isRoot:   isRoot,
		content:  map[string]disk.Content{},
		scanning: true,
		nextID:   1,
		bar:      progress.New(progress.WithDefaultGradient()),
	}
}

// SetSender wires the message send function (tea.Program.Send).
func (m *Model) SetSender(send func(tea.Msg)) { m.send = send }

// Shutdown interrupts running copies and waits for them to end.
func (m *Model) Shutdown() {
	for _, j := range m.jobs {
		j.Cancel()
	}
	for _, j := range m.jobs {
		j.Wait(15 * time.Second)
	}
}

func scanCmd() tea.Msg {
	disks, err := disk.List()
	content := map[string]disk.Content{}
	for _, d := range disks {
		content[d.Path] = disk.Inspect(d)
	}
	return scanMsg{disks: disks, content: content, err: err}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) Init() tea.Cmd { return tea.Batch(scanCmd, tick()) }

func (m *Model) rescan() tea.Cmd {
	m.scanning = true
	return scanCmd
}

func (m *Model) job(id int) *dd.Job {
	for _, j := range m.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

func (m *Model) running() int {
	n := 0
	for _, j := range m.jobs {
		if j.Running() {
			n++
		}
	}
	return n
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case scanMsg:
		m.scanning = false
		m.scanErr = msg.err
		m.disks = msg.disks
		m.content = msg.content
		if m.src != nil {
			if d := m.findDisk(m.src.Path); d != nil {
				m.src = d
			} else {
				m.src, m.state = nil, stSource
				m.status = tr("The source disk disappeared.", "Le disque source a disparu.")
			}
		}
		m.clampCursors()

	case tickMsg:
		return m, tick()

	case dd.ProgressMsg:
		if j := m.job(msg.ID); j != nil {
			j.Apply(msg)
		}

	case dd.DoneMsg:
		if j := m.job(msg.ID); j != nil {
			j.Finish(msg)
			switch j.Status {
			case dd.Done:
				m.status = sGreen.Render(tr("Copy #", "Copie #") + itoa(j.ID) + tr(" done.", " terminée."))
			case dd.Failed:
				m.status = sRed.Render(tr("Copy #", "Copie #") + itoa(j.ID) + tr(" failed.", " en erreur."))
			case dd.Canceled:
				m.status = sYellow.Render(tr("Copy #", "Copie #") + itoa(j.ID) + tr(" canceled.", " annulée."))
			}
		}
		return m, m.rescan()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) findDisk(path string) *disk.Disk {
	for i := range m.disks {
		if m.disks[i].Path == path {
			d := m.disks[i]
			return &d
		}
	}
	return nil
}

func (m *Model) clampCursors() {
	clamp := func(c, n int) int {
		if c >= n {
			c = n - 1
		}
		if c < 0 {
			c = 0
		}
		return c
	}
	m.srcCur = clamp(m.srcCur, len(m.sourceEntries()))
	m.dstCur = clamp(m.dstCur, len(m.destEntries()))
	m.jobCur = clamp(m.jobCur, len(m.jobs))
}

func (m *Model) askQuit() (tea.Model, tea.Cmd) {
	if m.running() == 0 {
		return m, tea.Quit
	}
	if m.state != stQuit {
		m.prev = m.state
	}
	m.state = stQuit
	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	// l switches the language, except while typing YES/OUI (F2 works everywhere).
	if key == "f2" || ((key == "l" || key == "L") && !m.typing()) {
		m.toggleLang()
		return m, nil
	}

	if m.state == stQuit {
		switch key {
		case "o", "O", "y", "Y", "ctrl+c":
			for _, j := range m.jobs {
				j.Cancel()
			}
			return m, tea.Quit
		case "n", "N", "esc":
			m.state = m.prev
		}
		return m, nil
	}
	if key == "ctrl+c" {
		return m.askQuit()
	}
	if m.state == stConfirm {
		return m.handleConfirmKey(k)
	}

	switch key {
	case "q":
		return m.askQuit()
	case "r":
		m.status = tr("Detecting disks…", "Détection des disques…")
		return m, m.rescan()
	case "tab":
		if m.state == stJobs {
			m.state = m.prev
		} else if len(m.jobs) > 0 {
			m.prev, m.state = m.state, stJobs
		}
		return m, nil
	}

	up := key == "up" || key == "k"
	down := key == "down" || key == "j"

	switch m.state {
	case stSource:
		entries := m.sourceEntries()
		switch {
		case up && m.srcCur > 0:
			m.srcCur--
		case down && m.srcCur < len(entries)-1:
			m.srcCur++
		case key == "enter" && m.srcCur < len(entries):
			e := entries[m.srcCur]
			if e.reason != "" {
				m.status = sRed.Render(e.d.Path + " : " + e.reason)
				break
			}
			d := e.d
			m.src, m.dstCur, m.state, m.status = &d, 0, stDest, ""
		}

	case stDest:
		entries := m.destEntries()
		switch {
		case up && m.dstCur > 0:
			m.dstCur--
		case down && m.dstCur < len(entries)-1:
			m.dstCur++
		case key == "esc" || key == "backspace":
			m.src, m.state, m.status = nil, stSource, ""
		case key == "enter" && m.dstCur < len(entries):
			e := entries[m.dstCur]
			if e.reason != "" {
				m.status = sRed.Render(e.d.Path + " : " + e.reason)
				break
			}
			d := e.d
			m.dst = &d
			m.openConfirm()
		}

	case stJobs:
		switch {
		case up && m.jobCur > 0:
			m.jobCur--
		case down && m.jobCur < len(m.jobs)-1:
			m.jobCur++
		case key == "esc":
			m.state = m.prev
		case key == "x" && m.jobCur < len(m.jobs):
			if j := m.jobs[m.jobCur]; j.Running() {
				j.Cancel()
				m.status = sYellow.Render(tr("Canceling copy #", "Annulation de la copie #") + itoa(j.ID) + "…")
			}
		case key == "d" && m.jobCur < len(m.jobs):
			if !m.jobs[m.jobCur].Running() {
				m.jobs = append(m.jobs[:m.jobCur], m.jobs[m.jobCur+1:]...)
				m.clampCursors()
				if len(m.jobs) == 0 {
					m.state = m.prev
				}
			}
		}
	}
	return m, nil
}

// typing is true while the confirmation dialog expects YES/OUI to be typed.
func (m *Model) typing() bool {
	return m.state == stConfirm && m.confirm.needOui && !m.confirm.blocked
}

// toggleLang switches between English and French and refreshes translated state.
func (m *Model) toggleLang() {
	l := i18n.Toggle()
	if m.state == stConfirm && m.src != nil && m.dst != nil {
		m.confirm.checks = m.buildChecks(m.confirm.srcC, m.confirm.dstC)
		m.confirm.msg = ""
	}
	m.status = sCyan.Render(tr("Language: ", "Langue : ") + l.Name())
}

// launch starts the confirmed copy and returns to source selection.
func (m *Model) launch() tea.Cmd {
	j := dd.NewJob(m.nextID, *m.src, *m.dst, m.confirm.plan)
	m.nextID++
	send := func(msg any) {
		if m.send != nil {
			m.send(msg)
		}
	}
	if err := j.Start(send); err != nil {
		j.Fail(err)
		m.status = sRed.Render(tr("Could not start dd: ", "Impossible de lancer dd : ") + err.Error())
	} else {
		m.status = sGreen.Render(tr("Copy #", "Copie #") + itoa(j.ID) + tr(" started: ", " lancée : ") + j.Src.Path + " → " + j.Dst.Path)
	}
	m.jobs = append(m.jobs, j)
	m.jobCur = len(m.jobs) - 1
	m.src, m.dst = nil, nil
	m.srcCur, m.dstCur = 0, 0
	m.state = stSource
	return m.rescan()
}

func (m *Model) View() string {
	if m.width == 0 {
		return tr("Loading…", "Chargement…")
	}
	w := m.width
	avail := m.height - 2
	topH := avail * 45 / 100
	if len(m.jobs) == 0 {
		topH = avail * 65 / 100
	}
	if m.state == stConfirm || m.state == stQuit {
		topH = avail * 65 / 100
	}
	if topH < 10 {
		topH = 10
	}
	jobsH := avail - topH
	if jobsH < 5 {
		jobsH = 5
	}

	var top string
	switch m.state {
	case stConfirm:
		top = m.renderConfirm(w, topH)
	case stQuit:
		top = m.renderQuit(w, topH)
	default:
		lw := w / 2
		active := m.state
		if active == stJobs {
			active = m.prev
		}
		left := m.renderList("① SOURCE", m.sourceEntries(), m.srcCur, active == stSource && m.state != stJobs, false, lw, topH)
		right := m.renderList("② DESTINATION", m.destEntries(), m.dstCur, active == stDest && m.state != stJobs, true, w-lw, topH)
		top = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(w),
		top,
		m.renderJobs(w, jobsH),
		m.renderHelp(w),
	)
}

func (m *Model) renderHeader(w int) string {
	left := sHeader.Render(tr("ddeck — disk cloning", "ddeck — clonage de disques"))
	if m.status != "" {
		left += "  " + m.status
	}
	right := sGreen.Render("root ✔")
	if !m.isRoot {
		right = sRed.Render(tr("⚠ not root: read-only, run with sudo", "⚠ pas root : lecture seule, lancez avec sudo"))
	}
	if m.scanning {
		right = sDim.Render(tr("scanning… ", "analyse… ")) + right
	}
	right = sKey.Render("["+i18n.Current().Code()+"]") + " " + right
	return fit([]string{spread(left, right+" ", w)}, w, 1)
}

func (m *Model) renderHelp(w int) string {
	var s string
	switch m.state {
	case stSource:
		s = help("↑/↓", tr("move", "naviguer"), tr("Enter", "Entrée"), tr("pick source", "choisir la source"),
			"Tab", tr("copies", "copies"), "r", tr("rescan", "rafraîchir"), "l", tr("Français", "English"), "q", tr("quit", "quitter"))
	case stDest:
		s = help("↑/↓", tr("move", "naviguer"), tr("Enter", "Entrée"), tr("pick destination", "choisir la destination"),
			tr("Esc", "Échap"), tr("back", "retour"), "Tab", tr("copies", "copies"), "r", tr("rescan", "rafraîchir"),
			"l", tr("Français", "English"), "q", tr("quit", "quitter"))
	case stConfirm:
		if m.confirm.blocked {
			s = help(tr("Esc", "Échap"), tr("back", "retour"), "l", tr("Français", "English"))
		} else if m.confirm.needOui {
			s = help(confirmWord()+tr(" + Enter", " + Entrée"), tr("force the copy", "forcer la copie"),
				tr("Esc", "Échap"), tr("back", "retour"), "F2", tr("Français", "English"))
		} else {
			s = help(tr("Y", "O"), tr("start the copy", "lancer la copie"), tr("N/Esc", "N/Échap"), tr("back", "retour"),
				"l", tr("Français", "English"))
		}
	case stJobs:
		s = help("↑/↓", tr("select", "sélectionner"), "x", tr("cancel copy", "annuler la copie"),
			"d", tr("remove from list", "retirer de la liste"), tr("Tab/Esc", "Tab/Échap"), tr("back", "retour"),
			"l", tr("Français", "English"), "q", tr("quit", "quitter"))
	case stQuit:
		s = help(tr("Y", "O"), tr("quit and cancel", "quitter et annuler"), "N", tr("stay", "rester"),
			"l", tr("Français", "English"))
	}
	return fit([]string{s}, w, 1)
}

func itoa(n int) string { return strconv.Itoa(n) }
