package ui

import (
	"fmt"
	"strings"

	"github.com/five82/flyer/internal/spindle"
)

// renderNowBand renders the live resource occupancy line under the header:
// which task of which item holds each scheduler resource, with live percent
// and fps/ETA where available. The drive segment always renders -- "insert
// the next disc" is the single most useful signal this UI carries.
func (m Model) renderNowBand() string {
	styles := m.theme.Styles()
	compact := m.width < compactWidthThreshold

	label := styles.FaintText.Bold(true).Render("NOW ")
	sep := " " + styles.RuleText.Render("|") + " "

	sched := m.snapshot.Status.Scheduler
	if sched == nil || len(sched.Resources) == 0 {
		// Old daemon or no scheduler info: fall back to a bare active count.
		if n := m.countProcessingItems(); n > 0 {
			return label + styles.MutedText.Render("Active:") + " " +
				styles.AccentText.Render(fmt.Sprintf("%d", n))
		}
		return label + styles.FaintText.Render("idle")
	}

	var parts []string
	for _, name := range resourceOrder(m.snapshot.Status.Pipeline, sched.Resources) {
		res := sched.Resources[name]
		rlabel := resourceLabel(name)

		if res.Used == 0 {
			// Idle resources stay quiet, except the drive.
			if name == "drive" {
				state, style := "FREE", styles.SuccessText
				if disc := m.snapshot.Status.Disc; disc != nil && disc.Paused {
					state, style = "PAUSED", styles.WarningText
				}
				parts = append(parts, styles.MutedText.Render(rlabel+":")+" "+
					style.Bold(true).Render(state))
			}
			continue
		}

		for _, h := range res.Holders {
			info := stageDisplay(h.Task)
			seg := styles.MutedText.Render(rlabel+":") + " " +
				styles.Text.Render(fmt.Sprintf("#%d", h.ItemID)) + " " +
				roleStyle(info.role, styles).Render(strings.ToLower(info.label))
			if !compact {
				for _, extra := range m.holderExtras(h) {
					seg += " " + styles.AccentText.Render(extra)
				}
			}
			parts = append(parts, seg)
		}
	}

	if len(parts) == 0 {
		return label + styles.FaintText.Render("idle")
	}
	return label + strings.Join(parts, sep)
}

// holderExtras returns live figures for a resource holder's running task:
// percent, and fps/ETA for encodes.
func (m Model) holderExtras(h spindle.ResourceHolder) []string {
	for i := range m.snapshot.Queue {
		item := &m.snapshot.Queue[i]
		if item.ID != h.ItemID {
			continue
		}
		for _, t := range item.Tasks {
			if t.Type != h.Task || !t.IsRunning() {
				continue
			}
			var extras []string
			if pct := t.Progress.Percent; pct > 0 {
				extras = append(extras, fmt.Sprintf("%.0f%%", pct))
			}
			if t.Type == "encoding" && item.Encoding != nil {
				if item.Encoding.FPS > 0 {
					extras = append(extras, fmt.Sprintf("%.0f fps", item.Encoding.FPS))
				}
				if eta := item.Encoding.ETADuration(); eta > 0 {
					extras = append(extras, "ETA "+formatDuration(eta))
				}
			}
			return extras
		}
	}
	return nil
}
