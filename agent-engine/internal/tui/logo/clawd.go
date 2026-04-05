package logo

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/wall-ai/agent-engine/internal/tui/color"
	"github.com/wall-ai/agent-engine/internal/tui/themes"
)

// LobsterPose identifies a mascot pose.
type LobsterPose string

const (
	PoseDefault  LobsterPose = "default"
	PoseClawUp   LobsterPose = "claw-up"
	PoseClawSnap LobsterPose = "claw-snap"
	PoseSwim     LobsterPose = "swim"
)

// lobsterFrame holds the multi-line ASCII art for one pose.
type lobsterFrame struct {
	lines []string
}

// LobsterWidth is the visual column width of the lobster ASCII art.
const LobsterWidth = 22

var lobsterPoses = map[LobsterPose]lobsterFrame{
	PoseDefault: {lines: []string{
		`   (\/)  (\/)        `,
		`    \/    \/         `,
		`   ,-.------.-.     `,
		`  / (  o  o  ) \    `,
		`  \  \  __  /  /    `,
		`   '-.\    /.-'     `,
		`    _/ \~~/ \_      `,
		`   {__/    \__}     `,
	}},
	PoseClawUp: {lines: []string{
		`  \(\/)    (\//     `,
		`   \/        \/     `,
		`   ,-.------.-.     `,
		`  / (  ^  ^  ) \    `,
		`  \  \  __  /  /    `,
		`   '-.\    /.-'     `,
		`    _/ \~~/ \_      `,
		`   {__/    \__}     `,
	}},
	PoseClawSnap: {lines: []string{
		`   (><)  (><)        `,
		`    \/    \/         `,
		`   ,-.------.-.     `,
		`  / (  o  o  ) \    `,
		`  \  \  \/  /  /    `,
		`   '-.\    /.-'     `,
		`    _/ \~~/ \_      `,
		`   {__/    \__}     `,
	}},
	PoseSwim: {lines: []string{
		`   (\/)  (\/)        `,
		`    \/    \/         `,
		`  ~,-.------.-.~    `,
		`  / (  -  -  ) \    `,
		`  \  \  __  /  /    `,
		` ~~'-.\    /.-'~~   `,
		`    _/ \~~/ \_      `,
		`   {__/    \__}     `,
	}},
}

// RenderLobster returns the multi-line lobster mascot string with ANSI colors.
func RenderLobster(pose LobsterPose, theme themes.Theme) string {
	f, ok := lobsterPoses[pose]
	if !ok {
		f = lobsterPoses[PoseDefault]
	}

	bodyC := color.Resolve(theme.LobsterBody)
	body := lipgloss.NewStyle().Foreground(bodyC)

	var sb strings.Builder
	for i, line := range f.lines {
		sb.WriteString(body.Render(line))
		if i < len(f.lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// RenderClawd is a backward-compatible alias for RenderLobster.
func RenderClawd(pose LobsterPose, theme themes.Theme) string {
	return RenderLobster(pose, theme)
}

// ClawdPose is a backward-compatible alias for LobsterPose.
type ClawdPose = LobsterPose
