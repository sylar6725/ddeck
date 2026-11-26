package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ddeck/internal/dd"
	"ddeck/internal/disk"
)

type level int

const (
	lvInfo  level = iota // plain warning
	lvWarn               // possible swap: requires typing YES/OUI
	lvBlock              // prevents the copy
)

type check struct {
	ok    bool
	label string
	lv    level
}

type confirmState struct {
	plan    dd.Plan
	checks  []check
	srcC    disk.Content
	dstC    disk.Content
	blocked bool
	needOui bool
	input   string
	msg     string
}

// openConfirm re-reads the real state of both disks, then prepares the dialog.
func (m *Model) openConfirm() {
	if disks, err := disk.List(); err == nil {
		m.disks = disks
	}
	src, dst := m.findDisk(m.src.Path), m.findDisk(m.dst.Path)
	if src == nil || dst == nil {
		m.status = sRed.Render(tr("Disk not found, rescan (r).", "Disque introuvable, rafraîchissez (r)."))
		m.src, m.dst, m.state = nil, nil, stSource
		return
	}
	m.src, m.dst = src, dst
	sc, dc := disk.Inspect(*src), disk.Inspect(*dst)
	m.content[src.Path], m.content[dst.Path] = sc, dc

	cs := confirmState{plan: dd.Optimize(*src, *dst), srcC: sc, dstC: dc}
	cs.checks = m.buildChecks(sc, dc)
	for _, c := range cs.checks {
		if !c.ok && c.lv == lvBlock {
			cs.blocked = true
		}
		if !c.ok && c.lv == lvWarn {
			cs.needOui = true
		}
	}
	m.confirm = cs
	m.state = stConfirm
}

// buildChecks lists the safety checks in the current language.
func (m *Model) buildChecks(sc, dc disk.Content) []check {
	src, dst := m.src, m.dst
	dstLabel := tr("The destination is empty", "La destination est vide")
	switch {
	case dc.HasData():
		dstLabel = tr("The destination contains data (", "La destination contient des données (") + dc.Summary() + ")"
	case !dc.Checked:
		dstLabel = tr("Destination content unreadable: cannot verify that it is empty",
			"Contenu de la destination illisible : impossible de vérifier qu'elle est vide")
	}
	srcLabel := tr("The source contains data (", "La source contient des données (") + sc.Summary() + ")"
	if !sc.HasData() {
		srcLabel = tr("The source looks empty (", "La source semble vide (") + sc.Summary() + ")"
	}
	mountLabel := tr("The destination is not mounted", "La destination n'est pas montée")
	if dst.Mounted() {
		mountLabel = tr("The destination is mounted (", "La destination est montée (") +
			strings.Join(dst.AllMountpoints(), ", ") + tr("): unmount it", ") : démontez-la")
	}
	srcMount := tr("The source is not mounted", "La source n'est pas montée")
	if src.Mounted() {
		srcMount = tr("The source is mounted: the copy may be inconsistent if it is modified",
			"La source est montée : la copie peut être incohérente si elle est modifiée")
	}

	return []check{
		{m.isRoot, tr("Root privileges (sudo)", "Droits root (sudo)"), lvBlock},
		{src.Path != dst.Path, tr("Source and destination are different", "Source et destination différentes"), lvBlock},
		{dst.Size >= src.Size, tr("Destination is large enough (", "Taille de la destination suffisante (") +
			humanBytes(dst.Size) + " ≥ " + humanBytes(src.Size) + ")", lvBlock},
		{!dst.IsSystem(), tr("The destination is not the system disk", "La destination n'est pas le disque système"), lvBlock},
		{!dst.Mounted(), mountLabel, lvBlock},
		{sc.HasData(), srcLabel, lvWarn},
		{dc.Empty(), dstLabel, lvWarn},
		{!src.Mounted(), srcMount, lvInfo},
	}
}

// confirmWord is the word to type to force a risky copy.
func confirmWord() string { return tr("YES", "OUI") }

func (m *Model) handleConfirmKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := &m.confirm
	back := func() (tea.Model, tea.Cmd) {
		m.dst, m.state = nil, stDest
		return m, nil
	}
	switch k.String() {
	case "esc":
		return back()
	}
	if c.blocked {
		return m, nil
	}
	if !c.needOui {
		switch k.String() {
		case "o", "O", "y", "Y":
			return m, m.launch()
		case "n", "N":
			return back()
		}
		return m, nil
	}
	switch k.Type {
	case tea.KeyEnter:
		if w := strings.ToUpper(strings.TrimSpace(c.input)); w == "YES" || w == "OUI" {
			return m, m.launch()
		}
		c.msg = tr("Type exactly "+confirmWord()+" to force the copy.", "Tapez exactement "+confirmWord()+" pour forcer la copie.")
	case tea.KeyBackspace:
		if len(c.input) > 0 {
			c.input = c.input[:len(c.input)-1]
		}
	case tea.KeyRunes:
		if len(c.input) < 8 {
			c.input += string(k.Runes)
		}
	}
	return m, nil
}

func (m *Model) diskLine(role string, d *disk.Disk, c disk.Content) string {
	return sBold.Render(role) + "  " + sCursor.Render(d.Path) + "  " + sCyan.Render(d.Kind().String()) +
		"  " + d.Label() + "  " + humanBytes(d.Size) + "  " + sDim.Render("["+c.Summary()+"]")
}

func (m *Model) renderConfirm(w, h int) string {
	c := m.confirm
	bw := w - 4
	if bw > 110 {
		bw = 110
	}
	lines := []string{
		sTitle.Render(tr("③ CONFIRM COPY", "③ CONFIRMATION DE LA COPIE")),
		"",
		m.diskLine("SOURCE     ", m.src, c.srcC),
		m.diskLine("DESTINATION", m.dst, c.dstC),
		"",
		sBold.Render(tr("Checks", "Vérifications")),
	}
	for _, ch := range c.checks {
		switch {
		case ch.ok:
			lines = append(lines, "  "+sGreen.Render("✔")+" "+ch.label)
		case ch.lv == lvInfo:
			lines = append(lines, "  "+sYellow.Render("⚠ "+ch.label))
		case ch.lv == lvWarn:
			lines = append(lines, "  "+sOrange.Render("✖ "+ch.label))
		default:
			lines = append(lines, "  "+sRed.Render("✖ "+ch.label))
		}
	}
	lines = append(lines, "",
		sBold.Render(tr("Tuning: ", "Optimisation : "))+c.plan.Desc,
		sDim.Render(c.plan.Command()),
		"",
	)
	border := cAccent
	switch {
	case c.blocked:
		border = cRed
		lines = append(lines, sRed.Render(tr("Copy not possible: fix the items in red. [Esc] back", "Copie impossible : corrigez les points en rouge. [Échap] retour")))
	case c.needOui:
		border = cOrange
		lines = append(lines,
			sOrange.Bold(true).Render(tr("⚠ RISK OF SWAPPING SOURCE / DESTINATION!", "⚠ RISQUE D'INVERSION SOURCE / DESTINATION !")),
			sRed.Render(tr("All data on "+m.dst.Path+" will be PERMANENTLY erased.", "Toutes les données de "+m.dst.Path+" seront DÉFINITIVEMENT effacées.")),
			tr("Type ", "Tapez ")+sBold.Render(confirmWord())+tr(" then Enter to force: ", " puis Entrée pour forcer : ")+sCursor.Render(c.input+"█"),
		)
		if c.msg != "" {
			lines = append(lines, sRed.Render(c.msg))
		}
	default:
		lines = append(lines,
			sRed.Render(tr("All data on "+m.dst.Path+" will be erased.", "Toutes les données de "+m.dst.Path+" seront effacées.")),
			sKey.Render(tr("[Y]", "[O]"))+tr(" start the copy    ", " lancer la copie    ")+sKey.Render("[N]")+tr(" back", " retour"),
		)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(border).
		Padding(0, 2).
		Width(bw - 2).
		Render(fit(lines, bw-6, h-2))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderQuit(w, h int) string {
	lines := []string{
		sOrange.Bold(true).Render(tr("Copies are running (", "Des copies sont en cours (") + itoa(m.running()) + ")."),
		"",
		tr("Quitting now will interrupt them: the destination disks will be incomplete.",
			"Quitter maintenant les interrompra : les disques de destination seront incomplets."),
		"",
		sKey.Render(tr("[Y]", "[O]")) + tr(" quit and cancel    ", " quitter et annuler    ") + sKey.Render("[N]") + tr(" stay", " rester"),
	}
	box := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(cOrange).Padding(1, 3).
		Render(fit(lines, w-10, h-4))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}
