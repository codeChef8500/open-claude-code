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
	spriteBody := buddy.RenderSprite(m.companion.CompanionBones, frame)

	// Blink: replace eye with '-'
	if m.animState == AnimBlink {
		eye := string(m.companion.Eye)
		for i, l := range spriteBody {
			spriteBody[i] = strings.ReplaceAll(l, eye, "-")
		}
	}

	// Prepend heart frames if petting (TS: PET_HEARTS[petAge % PET_HEARTS.length])
	petting := false
	if m.petAt > 0 {
		petAge := m.tick - m.petStartTick
		if petAge*TickMS < PetBurstMS && petAge >= 0 {
			petting = true
			heartIdx := petAge % len(HeartFrames)
			spriteBody = append([]string{HeartFrames[heartIdx]}, spriteBody...)
		}
	}

	// Rarity color for sprite lines (TS: each line gets rarity color;
	// heart line uses autoAccept instead)
	rarityColor := buddy.RarityHexColors[m.companion.Rarity]
	spriteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor))
	for i, l := range spriteBody {
		if i == 0 && petting {
			heartStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65"))
			spriteBody[i] = heartStyle.Render(l)
		} else {
			spriteBody[i] = spriteStyle.Render(l)
		}
	}

	// Build info column (placed to the right of sprite body, bottom-aligned)
	// This keeps total height = sprite body height, avoiding clipping.
	var infoLines []string
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rarityColor)).Faint(true)

	// Name
	name := m.companion.Name
	if name != "" {
		nameStyle := lipgloss.NewStyle().Italic(true)
		if m.focused {
			nameStyle = nameStyle.Bold(true).Reverse(true).Foreground(lipgloss.Color(rarityColor))
			name = " " + name + " "
		} else {
			nameStyle = nameStyle.Faint(true)
		}
		infoLines = append(infoLines, nameStyle.Render(name))
	}

	// Rarity stars + species
	stars := buddy.RarityStars[m.companion.Rarity]
	species := string(m.companion.Species)
	infoLines = append(infoLines, infoStyle.Render(stars+" "+species))

	// Top stat
	topStat := ""
	topVal := -1
	for _, sn := range buddy.AllStatNames {
		if v := m.companion.Stats[sn]; v > topVal {
			topVal = v
			topStat = statAbbrev[sn]
		}
	}
	if topStat != "" {
		infoLines = append(infoLines, infoStyle.Render(fmt.Sprintf("%s:%d", topStat, topVal)))
	}

	// Personality (truncated)
	if m.companion.Personality != "" {
		pers := m.companion.Personality
		const maxPersLen = 14
		if utf8.RuneCountInString(pers) > maxPersLen {
			pers = string([]rune(pers)[:maxPersLen-1]) + "…"
		}
		persStyle := lipgloss.NewStyle().Italic(true).Faint(true)
		infoLines = append(infoLines, persStyle.Render("\""+pers+"\""))
	}

	// Combine: sprite body (left) + info (right), bottom-aligned
	combined := joinSideBySide(spriteBody, infoLines, 2)

	// In fullscreen mode, the inline bubble is suppressed (handled by FloatingBubbleView).
	// In normal mode, render bubble to the left of the sprite+info block.
	if !m.fullscreen && m.IsSpeaking() {
		ticksSinceSpoke := m.tick - m.lastSpokeTick
		fading := ticksSinceSpoke >= (BubbleShow - FadeWindow)
		bubbleLines := RenderBubble(m.reaction, fading, TailRight)
		if len(bubbleLines) > 0 {
			return joinHorizontal(bubbleLines, combined, 1)
		}
	}

	return strings.Join(combined, "\n")
}

// joinSideBySide places left and right slices side-by-side,
// bottom-aligning the right column to the left column.
// Uses lipgloss.Width for ANSI-aware measurement.
func joinSideBySide(left, right []string, gap int) []string {
	leftW := 0
	for _, l := range left {
		if w := lipgloss.Width(l); w > leftW {
			leftW = w
		}
	}

	height := len(left)
	if len(right) > height {
		height = len(right)
	}

	padStr := strings.Repeat(" ", gap)
	rightOffset := height - len(right) // bottom-align right column

	result := make([]string, 0, height)
	for i := 0; i < height; i++ {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if ri := i - rightOffset; ri >= 0 && ri < len(right) {
			r = right[ri]
		}
		pad := leftW - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		result = append(result, l+strings.Repeat(" ", pad)+padStr+r)
	}
	return result
}

// joinHorizontal places left and right string slices side-by-side with a gap.
// Uses lipgloss.Width for ANSI-aware measurement.
func joinHorizontal(left, right []string, gap int) string {
	leftW := 0
	for _, l := range left {
		if w := lipgloss.Width(l); w > leftW {
			leftW = w
		}
	}

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
		pad := leftW - lipgloss.Width(l)
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

	// Petting: prepend heart with autoAccept color (TS: color="autoAccept")
	petting := false
	if m.petAt > 0 {
		petAge := m.tick - m.petStartTick
		if petAge*TickMS < PetBurstMS && petAge >= 0 {
			petting = true
		}
	}

	// Style the face with rarity color + bold
	faceStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(rarityColor))
	styledFace := ""
	if petting {
		heartStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4eba65"))
		styledFace = heartStyle.Render("♥") + " " + faceStyle.Render(face)
	} else {
		styledFace = faceStyle.Render(face)
	}

	// Build label: quip if speaking, otherwise name
	// TS: label = quip ? `"${quip}"` : focused ? ` ${name} ` : name
	quip := ""
	hasReaction := false
	if m.IsSpeaking() {
		hasReaction = true
		quip = m.reaction
		if utf8.RuneCountInString(quip) > narrowQuipCap {
			runes := []rune(quip)
			quip = string(runes[:narrowQuipCap-1]) + "…"
		}
	}

	// Fading state (needed for label color)
	fading := false
	if hasReaction {
		ticksSinceSpoke := m.tick - m.lastSpokeTick
		fading = ticksSinceSpoke >= (BubbleShow - FadeWindow)
	}

	var label string
	if quip != "" {
		label = "\"" + quip + "\""
	} else if m.focused {
		label = " " + name + " "
	} else {
		label = name
	}

	// Style the label — matches TS CompanionSprite.tsx line 236:
	//   dimColor={!focused && !reaction}
	//   bold={focused}
	//   inverse={focused && !reaction}
	//   color={reaction ? (fading ? 'inactive' : color) : (focused ? color : undefined)}
	var styledLabel string
	if label != "" {
		labelStyle := lipgloss.NewStyle().Italic(true)
		if m.focused {
			labelStyle = labelStyle.Bold(true)
			if !hasReaction {
				labelStyle = labelStyle.Reverse(true)
			}
			labelStyle = labelStyle.Foreground(lipgloss.Color(rarityColor))
		} else if hasReaction {
			if fading {
				labelStyle = labelStyle.Foreground(lipgloss.Color("#999999")) // inactive
			} else {
				labelStyle = labelStyle.Foreground(lipgloss.Color(rarityColor))
			}
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

// spriteBodyWidth is the standard width of the ASCII sprite art.
const spriteBodyWidth = 12

// SpriteColWidth returns the total visual width of the companion block
// (sprite body + gap + info column) for layout calculations.
func (m Model) SpriteColWidth() int {
	return m.spriteColWidth()
}

func (m Model) spriteColWidth() int {
	const infoGap = 2
	bodyW := spriteBodyWidth
	infoW := m.infoColWidth()
	if infoW == 0 {
		return bodyW
	}
	return bodyW + infoGap + infoW
}

// infoColWidth returns the max visual width of the info lines (name, stars+species, stat, etc.)
func (m Model) infoColWidth() int {
	if m.companion == nil {
		return 0
	}
	w := m.NameWidth()
	// Stars + species: "★★★ capybara"
	stars := buddy.RarityStars[m.companion.Rarity]
	species := string(m.companion.Species)
	if iw := utf8.RuneCountInString(stars) + 1 + utf8.RuneCountInString(species); iw > w {
		w = iw
	}
	// Personality line (truncated to 14 + 2 quotes)
	if m.companion.Personality != "" {
		persLen := utf8.RuneCountInString(m.companion.Personality)
		if persLen > 14 {
			persLen = 14
		}
		if pl := persLen + 2; pl > w { // +2 for quotes
			w = pl
		}
	}
	return w
}

// centerText pads a string to center it within width.
func centerText(s string, width int) string {
	w := utf8.RuneCountInString(s)
	if w >= width {
		return s
	}
	pad := (width - w) / 2
	return strings.Repeat(" ", pad) + s
}
