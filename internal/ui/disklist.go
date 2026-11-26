package ui

import (
	"fmt"
	"strings"

	"ddeck/internal/disk"
)

// entry is one row of a disk list; a non-empty reason means it cannot be selected.
type entry struct {
	d      disk.Disk
	reason string
}

func (m *Model) busy(path string) bool {
	for _, j := range m.jobs {
		if j.Running() && (j.Src.Path == path || j.Dst.Path == path) {
			return true
		}
	}
	return false
}

func (m *Model) sourceEntries() []entry {
	var out []entry
	for _, d := range m.disks {
		e := entry{d: d}
		if m.busy(d.Path) {
			e.reason = tr("copy running", "copie en cours")
		}
		out = append(out, e)
	}
	return out
}

func (m *Model) destEntries() []entry {
	if m.src == nil {
		return nil
	}
	var out []entry
	for _, d := range m.disks {
		if d.Path == m.src.Path {
			continue
		}
		e := entry{d: d}
		switch {
		case m.busy(d.Path):
			e.reason = tr("copy running", "copie en cours")
		case d.IsSystem():
			e.reason = tr("system disk", "disque système")
		case d.Size < m.src.Size:
			e.reason = tr("too small", "trop petit")
		}
		out = append(out, e)
	}
	return out
}

const entryLines = 3 // 2 info lines + 1 blank line

// renderList draws a disk list inside a panel.
func (m *Model) renderList(title string, entries []entry, cursor int, active, isDest bool, w, h int) string {
	ts := sTitleOff
	if active {
		ts = sTitle
	}
	lines := []string{ts.Render(title), ""}
	ih := h - 2 - len(lines)

	switch {
	case isDest && m.src == nil:
		lines = append(lines, sDim.Render(tr("Select the source disk first.", "Sélectionnez d'abord le disque source.")))
		return panel(lines, w, h, active)
	case m.scanning && len(m.disks) == 0:
		lines = append(lines, sDim.Render(tr("Detecting disks…", "Détection des disques…")))
		return panel(lines, w, h, active)
	case m.scanErr != nil:
		lines = append(lines, sRed.Render(tr("Error: ", "Erreur : ")+m.scanErr.Error()))
		return panel(lines, w, h, active)
	case len(entries) == 0:
		lines = append(lines, sDim.Render(tr("No disk available.", "Aucun disque disponible.")))
		return panel(lines, w, h, active)
	}

	per := ih / entryLines
	if per < 1 {
		per = 1
	}
	start := 0
	if cursor >= per {
		start = cursor - per + 1
	}
	if start > 0 {
		lines[1] = sDim.Render(fmt.Sprintf(tr("  ▲ %d more", "  ▲ %d de plus"), start))
	}
	end := start + per
	if end > len(entries) {
		end = len(entries)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.renderEntry(entries[i], i == cursor && active, isDest)...)
	}
	if end < len(entries) {
		lines = append(lines, sDim.Render(fmt.Sprintf(tr("  ▼ %d more", "  ▼ %d de plus"), len(entries)-end)))
	}
	return panel(lines, w, h, active)
}

func (m *Model) renderEntry(e entry, selected, isDest bool) []string {
	d := e.d
	marker := "  "
	pathStyle := sBold
	if selected {
		marker = sCursor.Render("▸ ")
		pathStyle = sCursor
	}
	if e.reason != "" {
		pathStyle = sDim
	}
	if !isDest && m.src != nil && m.src.Path == d.Path {
		marker = sGreen.Render("✔ ")
	}

	tags := []string{sCyan.Render(d.Kind().String()), humanBytes(d.Size)}
	if d.IsSystem() {
		tags = append(tags, sOrange.Render(tr("[system]", "[système]")))
	} else if d.Mounted() {
		tags = append(tags, sOrange.Render(tr("[mounted]", "[monté]")))
	}
	if d.Removable {
		tags = append(tags, sDim.Render(tr("removable", "amovible")))
	}
	line1 := marker + pathStyle.Render(d.Path) + "  " + strings.Join(tags, "  ")

	c, ok := m.content[d.Path]
	var badge string
	switch {
	case e.reason != "":
		badge = sRed.Render("✖ " + e.reason)
	case !ok:
		badge = sDim.Render(tr("… scanning", "… analyse"))
	case isDest && c.HasData():
		badge = sOrange.Render(tr("⚠ contains data (", "⚠ contient des données (") + c.Summary() + ")")
	case isDest && c.Empty():
		badge = sGreen.Render(tr("○ empty", "○ vide"))
	case c.HasData():
		badge = sGreen.Render(tr("● data (", "● données (") + c.Summary() + ")")
	case c.Empty():
		badge = sOrange.Render(tr("○ empty", "○ vide"))
	default:
		badge = sDim.Render(tr("? unknown content", "? contenu inconnu"))
	}
	label := d.Label()
	if d.Serial != "" && len(d.Serial) <= 20 {
		label += " · " + d.Serial
	}
	line2 := "    " + badge + "  " + sDim.Render(label)
	return []string{line1, line2, ""}
}
