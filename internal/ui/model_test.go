package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ddeck/internal/dd"
	"ddeck/internal/disk"
	"ddeck/internal/i18n"
)

func testModel() *Model {
	m := New(true)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	src := disk.Disk{Name: "nvme0n1", Path: "/dev/nvme0n1", Type: "disk", Tran: "nvme", Model: "Samsung 980", Size: 500e9}
	dst := disk.Disk{Name: "sdb", Path: "/dev/sdb", Type: "disk", Tran: "usb", Model: "Kingston", Size: 512e9}
	small := disk.Disk{Name: "sdc", Path: "/dev/sdc", Type: "disk", Tran: "usb", Size: 8e9}
	m.Update(scanMsg{disks: []disk.Disk{src, dst, small}, content: map[string]disk.Content{
		src.Path: {Checked: true, PTType: "gpt", FSTypes: []string{"ext4"}},
		dst.Path: {Checked: true},
	}})
	return m
}

func checkSize(t *testing.T, m *Model, name string) string {
	v := m.View()
	if h := lipgloss.Height(v); h > m.height {
		t.Errorf("%s : hauteur %d > %d", name, h, m.height)
	}
	if w := lipgloss.Width(v); w > m.width {
		t.Errorf("%s : largeur %d > %d", name, w, m.width)
	}
	return v
}

func TestFlow(t *testing.T) {
	m := testModel()
	v := checkSize(t, m, "source")
	if !strings.Contains(v, "/dev/nvme0n1") || !strings.Contains(v, "gpt: ext4") {
		t.Fatal("liste source incomplète")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stDest || m.src.Path != "/dev/nvme0n1" {
		t.Fatal("source non sélectionnée")
	}
	v = checkSize(t, m, "dest")
	if !strings.Contains(v, "too small") {
		t.Error("sdc devrait être trop petit")
	}
	for _, e := range m.destEntries() {
		if e.d.Path == "/dev/nvme0n1" {
			t.Error("la source ne doit pas être proposée en destination")
		}
	}
	// simulate the confirm screen without touching real devices
	m.dst = &m.destEntries()[0].d
	m.confirm = confirmState{plan: dd.Optimize(*m.src, *m.dst), needOui: true,
		checks: []check{{false, "La destination contient des données", lvWarn}}}
	m.state = stConfirm
	v = checkSize(t, m, "confirm")
	if !strings.Contains(v, "YES") {
		t.Error("OUI attendu")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("non")})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stConfirm || m.confirm.msg == "" {
		t.Error("une saisie autre que OUI ne doit pas lancer la copie")
	}

	// jobs panel with several copies
	for i := 0; i < 5; i++ {
		j := dd.NewJob(i+1, *m.src, *m.dst, m.confirm.plan)
		j.Apply(dd.ProgressMsg{ID: j.ID, Bytes: int64(i) * 50e9, Elapsed: float64(i + 1)})
		m.jobs = append(m.jobs, j)
	}
	m.state = stSource
	v = checkSize(t, m, "jobs")
	if !strings.Contains(v, "ETA") || !strings.Contains(v, "#5") {
		t.Error("panneau des copies incomplet")
	}
}

func TestLanguageToggle(t *testing.T) {
	defer i18n.Set(i18n.EN)
	m := testModel()
	if v := checkSize(t, m, "en"); !strings.Contains(v, "disk cloning") {
		t.Fatal("English expected by default")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if i18n.Current() != i18n.FR {
		t.Fatal("l should switch to French")
	}
	if v := checkSize(t, m, "fr"); !strings.Contains(v, "clonage de disques") {
		t.Error("French interface expected")
	}

	// while typing OUI/YES, l is a letter, F2 still toggles
	m.state = stConfirm
	m.confirm = confirmState{needOui: true}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if i18n.Current() != i18n.FR || m.confirm.input != "l" {
		t.Error("l must be typed into the confirmation field")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyF2})
	if i18n.Current() != i18n.EN {
		t.Error("F2 should switch back to English")
	}
}
