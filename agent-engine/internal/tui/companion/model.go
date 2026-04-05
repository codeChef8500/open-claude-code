package companion

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wall-ai/agent-engine/internal/buddy"
)

// statAbbrev maps stat names to 3-char abbreviations for compact display.
var statAbbrev = map[buddy.StatName]string{
	buddy.StatDebugging: "DBG",
	buddy.StatPatience:  "PAT",
	buddy.StatChaos:     "CHS",
	buddy.StatWisdom:    "WIS",
	buddy.StatSnark:     "SNK",
}

// ─── Tick message ────────────────────────────────────────────────────────────

// TickMsg is sent every TickDuration to advance animation.
type TickMsg time.Time

// ─── Model ───────────────────────────────────────────────────────────────────

// Model is the Bubbletea sub-model for the companion sprite.
type Model struct {
	companion *buddy.Companion
	width     int // terminal columns available

	// Animation state
	tick          int
	lastSpokeTick int // tick when reaction appeared
	petStartTick  int // tick when petting started
	animState     AnimState

	// External state (set by parent)
	reaction   string // current speech bubble text
	petAt      int64  // Unix ms of last pet
	muted      bool
	focused    bool // footer navigation focus
	fullscreen bool // true when app is in fullscreen mode
}

// New creates a companion Model.
func New() Model {
	return Model{
		animState: AnimIdle,
	}
}

// SetCompanion updates the companion data.
func (m *Model) SetCompanion(c *buddy.Companion) {
	m.companion = c
}

// SetWidth sets the available terminal width.
func (m *Model) SetWidth(w int) {
	m.width = w
}

// SetReaction sets the speech bubble text. Empty clears it.
func (m *Model) SetReaction(text string) {
	if text != m.reaction {
		m.reaction = text
		if text != "" {
			m.lastSpokeTick = m.tick
		}
	}
}

// SetPetAt triggers the petting heart animation.
func (m *Model) SetPetAt(ts int64) {
	if ts != m.petAt {
		m.petAt = ts
		m.petStartTick = m.tick
	}
}

// SetMuted sets the muted state.
func (m *Model) SetMuted(muted bool) {
	m.muted = muted
}

// SetFocused sets focus state.
func (m *Model) SetFocused(f bool) {
	m.focused = f
}

// SetFullscreen sets whether the app is in fullscreen mode.
func (m *Model) SetFullscreen(fs bool) {
	m.fullscreen = fs
}

// IsVisible returns true if the companion should be rendered.
func (m *Model) IsVisible() bool {
	return m.companion != nil && !m.muted
}

// ─── Bubbletea interface ─────────────────────────────────────────────────────

// Init starts the tick timer.
func (m Model) Init() tea.Cmd {
	return tea.Tick(TickDuration, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles tick messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		m.tick++
		m.updateAnimState()
		return m, tea.Tick(TickDuration, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})
	}
	return m, nil
}

// updateAnimState determines the current animation mode.
func (m *Model) updateAnimState() {
	// Check if petting is active
	isPetting := false
	if m.petAt > 0 {
		elapsed := (m.tick - m.petStartTick) * TickMS
		isPetting = elapsed < PetBurstMS
	}

	// Check if reaction bubble is active
	hasReaction := m.reaction != "" && (m.tick-m.lastSpokeTick) < BubbleShow

	if isPetting || hasReaction {
		m.animState = AnimExcite
	} else {
		// Idle sequence
		idx := m.tick % len(IdleSequence)
		if IdleSequence[idx] == -1 {
			m.animState = AnimBlink
		} else {
			m.animState = AnimIdle
		}
	}

	// Auto-clear reaction after BubbleShow ticks
	if m.reaction != "" && (m.tick-m.lastSpokeTick) >= BubbleShow {
		m.reaction = ""
	}
}

// View renders the companion widget.
func (m Model) View() string {
	if !m.IsVisible() {
		return ""
	}

	if m.width < MinColsFull {
		return m.renderNarrow()
	}
	return m.renderFull()
}

// ─── Rendering ───────────────────────────────────────────────────────────────

func (m Model) currentFrame() int {
	switch m.animState {
	case AnimExcite:
		// Fast cycle through all frames
		fc := buddy.SpriteFrameCount(m.companion.Species)
		return m.tick % fc
	case AnimBlink:
		return 0 // blink uses frame 0 with eye replacement
	default:
		idx := m.tick % len(IdleSequence)
		f := IdleSequence[idx]
		if f < 0 {
			f = 0
		}
		return f
	}
}

// NameWidth returns the rune count of the companion name (for layout).
func (m Model) NameWidth() int {
	if m.companion == nil {
		return 0
	}
	return utf8.RuneCountInString(m.companion.Name)
}

// IsSpeaking returns true if the companion has an active reaction bubble.
func (m Model) IsSpeaking() bool {
	return m.reaction != "" && (m.tick-m.lastSpokeTick) < BubbleShow
}

// IsFullscreen returns whether the model is in fullscreen mode.
func (m Model) IsFullscreen() bool {
	return m.fullscreen
}

// FloatingBubbleView renders a standalone floating speech bubble (for fullscreen mode).
// Returns empty string if no active reaction or not in fullscreen.
func (m Model) FloatingBubbleView() string {
	if !m.IsVisible() || !m.fullscreen || !m.IsSpeaking() {
		return ""
	}
	ticksSinceSpoke := m.tick - m.lastSpokeTick
	fading := ticksSinceSpoke >= (BubbleShow - FadeWindow)
	bubbleLines := RenderBubble(m.reaction, fading, TailDown)
	if len(bubbleLines) == 0 {
		return ""
	}
	return strings.Join(bubbleLines, "\n")
}

func (m Model) renderFull() string {
	frame := m.currentFrame()
	spriteLines := buddy.RenderSprite(m.companion.CompanionBones, frame)

	// Blink: replace eye with '-'
	if m.animState == AnimBlink {
		eye := string(m.companion.Eye)
		for i, l := range spriteLines {
			spriteLines[i] = strings.ReplaceAll(l, eye, "-")
		}
	}

	// Prepend heart frames if petting
	if m.petAt > 0 {
		elapsed := (m.tick - m.petStartTick) * TickMS
		if elapsed < PetBurstMS && elapsed >= 0 {
			heartIdx := elapsed / (PetBurstMS / len(HeartFrames))
			if heartIdx >= len(HeartFrames) {
				heartIdx = len(HeartFrames) - 1
			}
			spriteLines = append([]string{HeartFrames[heartIdx]}, spriteLines...)
		}
	}

	// Rarity color for sprite lines
	rarityColor := buddy.RarityHexColors[m.companion.Rarity]
	spriteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor))
	for i, l := range spriteLines {
		spriteLines[i] = spriteStyle.Render(l)
	}

	// Add styled name below sprite
	name := m.companion.Name
	if name != "" {
		nameStyle := lipgloss.NewStyle().Italic(true)
		if m.focused {
			nameStyle = nameStyle.Bold(true).Reverse(true)
			name = " " + name + " "
		} else {
			nameStyle = nameStyle.Foreground(lipgloss.Color(rarityColor)).Faint(true)
		}
		spriteLines = append(spriteLines, centerText(nameStyle.Render(name), 12))
	}

	// Rarity stars below name
	stars := buddy.RarityStars[m.companion.Rarity]
	starStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor))
	spriteLines = append(spriteLines, centerText(starStyle.Render(stars+" "+string(m.companion.Rarity)), 12))

	// Stats mini bars below rarity
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor))
	dimStyle := lipgloss.NewStyle().Faint(true)
	for _, sn := range buddy.AllStatNames {
		val := m.companion.Stats[sn]
		filled := val * 6 / 100
		if filled > 6 {
			filled = 6
		}
		empty := 6 - filled
		bar := barStyle.Render(strings.Repeat("█", filled)) + dimStyle.Render(strings.Repeat("░", empty))
		abbr := statAbbrev[sn]
		line := fmt.Sprintf("%s %s %2d", abbr, bar, val)
		spriteLines = append(spriteLines, line)
	}

	// In fullscreen mode, the inline bubble is suppressed (handled by FloatingBubbleView).
	// In normal mode, render bubble to the left of the sprite (side-by-side).
	if !m.fullscreen && m.IsSpeaking() {
		ticksSinceSpoke := m.tick - m.lastSpokeTick
		fading := ticksSinceSpoke >= (BubbleShow - FadeWindow)
		bubbleLines := RenderBubble(m.reaction, fading, TailRight)
		if len(bubbleLines) > 0 {
			return joinHorizontal(bubbleLines, spriteLines, 1)
		}
	}

	return strings.Join(spriteLines, "\n")
}

// joinHorizontal places left and right string slices side-by-side with a gap.
func joinHorizontal(left, right []string, gap int) string {
	// Find max width of left block
	leftW := 0
	for _, l := range left {
		w := utf8.RuneCountInString(l)
		if w > leftW {
			leftW = w
		}
	}

	// Determine total height
	height := len(left)
	if len(right) > height {
		height = len(right)
	}

	padStr := strings.Repeat(" ", gap)
	var sb strings.Builder
	for i := 0; i < height; i++ {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		// Pad left line to leftW
		pad := leftW - utf8.RuneCountInString(l)
		if pad < 0 {
			pad = 0
		}
		sb.WriteString(l)
		sb.WriteString(strings.Repeat(" ", pad))
		sb.WriteString(padStr)
		sb.WriteString(r)
		if i < height-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// narrowQuipCap is the max character count for a quip in narrow mode.
const narrowQuipCap = 24

func (m Model) renderNarrow() string {
	face := buddy.RenderFace(m.companion.CompanionBones)
	name := m.companion.Name
	rarityColor := buddy.RarityHexColors[m.companion.Rarity]

	// Blink: replace eye with '-'
	if m.animState == AnimBlink {
		eye := string(m.companion.Eye)
		face = strings.ReplaceAll(face, eye, "-")
	}

	// Petting: prepend heart
	if m.petAt > 0 {
		elapsed := (m.tick - m.petStartTick) * TickMS
		if elapsed < PetBurstMS && elapsed >= 0 {
			face = "♥ " + face
		}
	}

	// Style the face with rarity color + bold
	faceStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(rarityColor))
	styledFace := faceStyle.Render(face)

	// Build label: quip if speaking, otherwise name
	quip := ""
	if m.IsSpeaking() {
		quip = m.reaction
		if utf8.RuneCountInString(quip) > narrowQuipCap {
			runes := []rune(quip)
			quip = string(runes[:narrowQuipCap-1]) + "…"
		}
	}

	var label string
	if quip != "" {
		label = "\"" + quip + "\""
	} else {
		label = name
	}

	// Style the label
	var styledLabel string
	if label != "" {
		labelStyle := lipgloss.NewStyle().Italic(true)
		if m.focused {
			labelStyle = labelStyle.Bold(true).Reverse(true)
		} else if quip != "" {
			labelStyle = labelStyle.Foreground(lipgloss.Color(rarityColor))
		} else {
			labelStyle = labelStyle.Faint(true)
		}
		styledLabel = " " + labelStyle.Render(label)
	}

	// Append top stat in narrow mode: "WIS:81"
	topStat := ""
	topVal := -1
	for _, sn := range buddy.AllStatNames {
		if v := m.companion.Stats[sn]; v > topVal {
			topVal = v
			topStat = statAbbrev[sn]
		}
	}
	stars := buddy.RarityStars[m.companion.Rarity]
	statStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor)).Faint(true)
	suffix := " " + statStyle.Render(fmt.Sprintf("%s %s:%d", stars, topStat, topVal))

	return styledFace + styledLabel + suffix
}

// centerText pads a string to center it within width.
func centerText(s string, width int) string {
	if len(s) >= width {
		return s
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}
