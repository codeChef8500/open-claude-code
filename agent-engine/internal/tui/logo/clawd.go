package logo

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// ClawdPose identifies a mascot pose.
type ClawdPose string

const (
	PoseDefault   ClawdPose = "default"
	PoseLookLeft  ClawdPose = "look-left"
	PoseLookRight ClawdPose = "look-right"
	PoseArmsUp    ClawdPose = "arms-up"
)

// pose segments — each row is split so only eyes/arms vary.
type segments struct {
	r1L, r1E, r1R string // row 1: left-arm, eyes(bg), right-arm
	r2L, r2R      string // row 2: left-arm, right-arm (body is fixed █████)
}

var poses = map[ClawdPose]segments{
	PoseDefault: {
		r1L: " ▐", r1E: "▛███▜", r1R: "▌",
		r2L: "▝▜", r2R: "▛▘",
	},
	PoseLookLeft: {
		r1L: " ▐", r1E: "▟███▟", r1R: "▌",
		r2L: "▝▜", r2R: "▛▘",
	},
	PoseLookRight: {
		r1L: " ▐", r1E: "▙███▙", r1R: "▌",
		r2L: "▝▜", r2R: "▛▘",
	},
	PoseArmsUp: {
		r1L: "▗▟", r1E: "▛███▜", r1R: "▙▖",
		r2L: " ▜", r2R: "▛ ",
	},
}

// RenderClawd returns the 3-line Clawd mascot string with ANSI colors.
func RenderClawd(pose ClawdPose, theme themes.Theme) string {
	p, ok := poses[pose]
	if !ok {
		p = poses[PoseDefault]
	}

	bodyC := color.Resolve(theme.ClawdBody)
	bgC := color.Resolve(theme.ClawdBackground)

	body := lipgloss.NewStyle().Foreground(bodyC)
	bodyBg := lipgloss.NewStyle().Foreground(bodyC).Background(bgC)

	var sb strings.Builder

	// Row 1: arm + eyes(bg) + arm
	sb.WriteString(body.Render(p.r1L))
	sb.WriteString(bodyBg.Render(p.r1E))
	sb.WriteString(body.Render(p.r1R))
	sb.WriteString("\n")

	// Row 2: arm + body(bg) + arm
	sb.WriteString(body.Render(p.r2L))
	sb.WriteString(bodyBg.Render("█████"))
	sb.WriteString(body.Render(p.r2R))
	sb.WriteString("\n")

	// Row 3: feet (no background)
	sb.WriteString(body.Render("  ▘▘ ▝▝  "))

	return sb.String()
}
