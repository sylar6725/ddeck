package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	cAccent = lipgloss.Color("63")
	cGreen  = lipgloss.Color("42")
	cRed    = lipgloss.Color("196")
	cOrange = lipgloss.Color("214")
	cYellow = lipgloss.Color("220")
	cGray   = lipgloss.Color("241")
	cCyan   = lipgloss.Color("45")

	sTitle    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sTitleOff = lipgloss.NewStyle().Bold(true).Foreground(cGray)
	sBold     = lipgloss.NewStyle().Bold(true)
	sDim      = lipgloss.NewStyle().Foreground(cGray)
	sGreen    = lipgloss.NewStyle().Foreground(cGreen)
	sRed      = lipgloss.NewStyle().Foreground(cRed)
	sOrange   = lipgloss.NewStyle().Foreground(cOrange)
	sYellow   = lipgloss.NewStyle().Foreground(cYellow)
	sCyan     = lipgloss.NewStyle().Foreground(cCyan)
	sCursor   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	sHeader   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(cAccent).Padding(0, 1)
	sHelp     = lipgloss.NewStyle().Foreground(cGray)
	sKey      = lipgloss.NewStyle().Foreground(cAccent).Bold(true)
)

// panel dessine un cadre de taille exacte w×h ; le contenu est tronqué au besoin.
func panel(lines []string, w, h int, active bool) string {
	border := cGray
	if active {
		border = cAccent
	}
	iw, ih := w-4, h-2 // bordure + padding horizontal de 1
	if iw < 1 || ih < 1 {
		return ""
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(w - 2).
		Height(ih).
		Render(fit(lines, iw, ih))
}

// fit tronque chaque ligne à w colonnes et garde au plus h lignes.
func fit(lines []string, w, h int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Truncate(l, w, "…")
	}
	return strings.Join(out, "\n")
}

// spread place left à gauche et right à droite sur w colonnes.
func spread(left, right string, w int) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + right
}

// humanBytes formate en unités décimales (comme les fabricants de disques).
func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

func humanSpeed(bps float64) string {
	if bps <= 0 {
		return "— B/s"
	}
	return humanBytes(int64(bps)) + "/s"
}

func fmtDur(d time.Duration) string {
	s := int(d.Round(time.Second).Seconds())
	h, m, sec := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

// help formate une barre d'aide « touche action · touche action ».
func help(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, sKey.Render(pairs[i])+" "+sHelp.Render(pairs[i+1]))
	}
	return " " + strings.Join(parts, sHelp.Render("  ·  "))
}
