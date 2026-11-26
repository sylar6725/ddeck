package dd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ddeck/internal/disk"
)

// TestRealDD copie un fichier avec le vrai dd (fichiers ordinaires, pas de disques).
func TestRealDD(t *testing.T) {
	if _, err := os.Stat("/bin/dd"); err != nil {
		t.Skip("dd absent")
	}
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.img"), filepath.Join(dir, "dst.img")
	data := bytes.Repeat([]byte("ddeck!"), 3<<20)
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := disk.Disk{Path: src, Type: "loop", Size: int64(len(data))}
	d := disk.Disk{Path: dst, Type: "loop", Size: int64(len(data))}
	plan := Optimize(s, d)
	plan.Args = append(plan.Args[:4], "conv=fsync", "status=progress") // pas de direct I/O sur tmpfs
	j := NewJob(1, s, d, plan)
	msgs := make(chan any, 100)
	if err := j.Start(func(m any) { msgs <- m }); err != nil {
		t.Fatal(err)
	}
	timeout := time.After(30 * time.Second)
	for {
		select {
		case m := <-msgs:
			switch m := m.(type) {
			case ProgressMsg:
				j.Apply(m)
			case DoneMsg:
				j.Finish(m)
				if j.Status != Done {
					t.Fatalf("statut %v : %s", j.Status, j.Err)
				}
				got, _ := os.ReadFile(dst)
				if !bytes.Equal(got, data) {
					t.Fatal("copie différente")
				}
				return
			}
		case <-timeout:
			t.Fatal("timeout")
		}
	}
}
