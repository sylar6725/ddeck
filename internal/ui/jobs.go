package ui

import (
	"fmt"

	"ddeck/internal/dd"
)

const jobLines = 4 // 3 info lines + 1 blank line

func (m *Model) renderJobs(w, h int) string {
	active := m.state == stJobs
	ts := sTitleOff
	if active {
		ts = sTitle
	}
	title := "COPIES"
	if n := m.running(); n > 0 {
		title += fmt.Sprintf(tr("  (%d running)", "  (%d en cours)"), n)
	}
	lines := []string{ts.Render(title), ""}
	iw, ih := w-4, h-2-len(lines)

	if len(m.jobs) == 0 {
		lines = append(lines, sDim.Render(tr("No copies yet. Pick a source, then a destination, to start dd.", "Aucune copie. Choisissez une source puis une destination pour lancer dd.")))
		return panel(lines, w, h, active)
	}

	per := (ih + 1) / jobLines // the last blank line may be cut off
	if per < 1 {
		per = 1
	}
	// Without focus, follow the most recent copy; with focus, the selected one.
	focus := len(m.jobs) - 1
	if active {
		focus = m.jobCur
	}
	start := 0
	if focus >= per {
		start = focus - per + 1
	}
	if start > 0 {
		lines[1] = sDim.Render(fmt.Sprintf(tr("  ▲ %d more copy(ies) above", "  ▲ %d copie(s) au-dessus"), start))
	}
	end := start + per
	if end > len(m.jobs) {
		end = len(m.jobs)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.renderJob(m.jobs[i], active && i == m.jobCur, iw)...)
	}
	if end < len(m.jobs) {
		lines = append(lines, sDim.Render(fmt.Sprintf(tr("  ▼ %d more copy(ies) below", "  ▼ %d copie(s) en dessous"), len(m.jobs)-end)))
	}
	return panel(lines, w, h, active)
}

func (m *Model) renderJob(j *dd.Job, selected bool, w int) []string {
	gutter := "  "
	if selected {
		gutter = sCursor.Render("▌ ")
	}

	var status string
	switch j.Status {
	case dd.Running:
		if j.Syncing() {
			status = sCyan.Render(tr("● Syncing…", "● Synchronisation…"))
		} else {
			status = sCyan.Render(tr("● Running", "● En cours"))
		}
	case dd.Done:
		status = sGreen.Render(tr("✔ Done", "✔ Terminée"))
	case dd.Failed:
		status = sRed.Render(tr("✖ Error", "✖ Erreur"))
	case dd.Canceled:
		status = sYellow.Render(tr("■ Canceled", "■ Annulée"))
	}

	idStyle := sBold
	if selected {
		idStyle = sCursor
	}
	head := idStyle.Render(fmt.Sprintf("#%d", j.ID)) + "  " +
		sBold.Render(j.Src.Path) + sDim.Render(fmt.Sprintf(" (%s, %s)", j.Src.Label(), humanBytes(j.Src.Size))) +
		"  →  " +
		sBold.Render(j.Dst.Path) + sDim.Render(fmt.Sprintf(" (%s, %s)", j.Dst.Label(), humanBytes(j.Dst.Size)))
	line1 := gutter + spread(head, status, w-2)

	bar := m.bar
	bar.Width = w - 6
	if bar.Width < 10 {
		bar.Width = 10
	}
	line2 := gutter + "  " + bar.ViewAs(j.Percent())

	var line3 string
	if j.Status == dd.Failed {
		line3 = sRed.Render(j.Err)
	} else {
		eta := "—"
		switch {
		case j.Status == dd.Done:
			eta = "00:00"
		case j.ETA() > 0:
			eta = fmtDur(j.ETA())
		}
		speed := j.Speed
		if j.Status != dd.Running && j.Elapsed() > 0 {
			speed = float64(j.Bytes) / j.Elapsed().Seconds() // final average speed
		}
		line3 = fmt.Sprintf("%s / %s    %s    %s %s    %s %s    %s",
			sBold.Render(humanBytes(j.Bytes)), humanBytes(j.Total),
			sBold.Render(humanSpeed(speed)),
			sDim.Render("ETA"), sBold.Render(eta),
			sDim.Render(tr("elapsed", "écoulé")), fmtDur(j.Elapsed()),
			sDim.Render(j.Plan.Desc))
	}
	line3 = gutter + "  " + line3
	return []string{line1, line2, line3, ""}
}
