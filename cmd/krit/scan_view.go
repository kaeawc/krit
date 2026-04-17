package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/kaeawc/krit/internal/onboarding"
)

func (m initModel) viewScanning() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("krit init"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("target: %s", m.target)))
	b.WriteString("\n\n")

	// Progress bar for the strict scan's sub-stages. Only meaningful
	// while strict is running or just after it finishes; before then,
	// idx is 0 and the bar is empty. When scanCursor has moved past
	// strict, the bar snaps full.
	total := m.strictStageTotal
	if total <= 0 {
		total = len(onboarding.StrictStages)
	}
	idx := m.strictStageIdx
	if idx > total {
		idx = total
	}
	pct := float64(0)
	if total > 0 {
		pct = float64(idx) / float64(total)
	}
	bar := m.scanProgress.ViewAs(pct)

	label := m.strictStageLabel
	if label == "" {
		label = "starting strict scan..."
	}
	elapsed := durationLabel(m.elapsedSince(m.profileStart["strict"]))
	b.WriteString(fmt.Sprintf("  strict  %s  %d/%d  %s\n", bar, idx, total, elapsed))
	b.WriteString(dimStyle.Render(fmt.Sprintf("          %s", label)))
	b.WriteString("\n\n")

	// Per-profile status list. Strict is shown with its completed
	// duration once finished; the remaining three profiles show
	// pending / scanning / done + their individual durations.
	for i, p := range m.profiles {
		var status string
		switch {
		case i < m.scanCursor:
			res := m.scans[p]
			findings := 0
			if res != nil {
				findings = res.Total
			}
			status = accentStyle.Render(fmt.Sprintf("done (%d findings, %s)",
				findings, durationLabel(m.profileDurations[p])))
		case i == m.scanCursor:
			status = fmt.Sprintf("▸ scanning... (%s)",
				durationLabel(m.elapsedSince(m.profileStart[p])))
		default:
			status = dimStyle.Render("pending")
		}
		b.WriteString(fmt.Sprintf("  %-14s %s\n", p, status))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("q to cancel"))
	return b.String()
}

// elapsedSince returns how long ago start was, using m.now (which is
// refreshed by tickMsg) so the view advances smoothly while a scan
// runs. If start is zero, returns zero.
func (m initModel) elapsedSince(start time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	ref := m.now
	if ref.IsZero() {
		ref = time.Now()
	}
	if ref.Before(start) {
		return 0
	}
	return ref.Sub(start)
}

// durationLabel formats a duration as Xms (under 1s), X.Xs (under
// 1m), or XmY.Ys (1m+). Zero renders as a dash.
func durationLabel(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := d.Seconds() - float64(mins)*60
	return fmt.Sprintf("%dm%.1fs", mins, secs)
}
