package color

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RGB holds red, green, blue components (0-255).
type RGB struct {
	R, G, B int
}

// ParseRGB parses "rgb(r,g,b)" into an RGB struct.
// Returns ok=false if the string is not in rgb() format.
func ParseRGB(s string) (RGB, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "rgb(") || !strings.HasSuffix(s, ")") {
		return RGB{}, false
	}
	inner := s[4 : len(s)-1]
	parts := strings.Split(inner, ",")
	if len(parts) != 3 {
		return RGB{}, false
	}
	r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	g, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	b, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil {
		return RGB{}, false
	}
	return RGB{R: r, G: g, B: b}, true
}

// ToHex converts an RGB to a "#RRGGBB" hex string.
func (c RGB) ToHex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// ToRGBString converts an RGB to "rgb(r,g,b)" format.
func (c RGB) ToRGBString() string {
	return fmt.Sprintf("rgb(%d,%d,%d)", c.R, c.G, c.B)
}

// Interpolate blends two RGB colors. t=0 returns c1, t=1 returns c2.
func Interpolate(c1, c2 RGB, t float64) RGB {
	if t <= 0 {
		return c1
	}
	if t >= 1 {
		return c2
	}
	return RGB{
		R: int(float64(c1.R) + (float64(c2.R)-float64(c1.R))*t),
		G: int(float64(c1.G) + (float64(c2.G)-float64(c1.G))*t),
		B: int(float64(c1.B) + (float64(c2.B)-float64(c1.B))*t),
	}
}

// ansiNames maps "ansi:xxx" names to ANSI 16-color numbers.
var ansiNames = map[string]string{
	"black":         "0",
	"red":           "1",
	"green":         "2",
	"yellow":        "3",
	"blue":          "4",
	"magenta":       "5",
	"cyan":          "6",
	"white":         "7",
	"blackBright":   "8",
	"redBright":     "9",
	"greenBright":   "10",
	"yellowBright":  "11",
	"blueBright":    "12",
	"magentaBright": "13",
	"cyanBright":    "14",
	"whiteBright":   "15",
}

// Resolve converts a theme color string to a lipgloss.Color.
// Supported formats:
//   - "rgb(r,g,b)"      → lipgloss.Color("#rrggbb")
//   - "#rrggbb"          → lipgloss.Color("#rrggbb")
//   - "ansi:colorName"   → lipgloss.Color("0"-"15")
//   - "ansi256(n)"       → lipgloss.Color("n")
//   - plain number "123" → lipgloss.Color("123")
func Resolve(s string) lipgloss.Color {
	s = strings.TrimSpace(s)

	// rgb(r,g,b)
	if rgb, ok := ParseRGB(s); ok {
		return lipgloss.Color(rgb.ToHex())
	}

	// #hex
	if strings.HasPrefix(s, "#") {
		return lipgloss.Color(s)
	}

	// ansi:colorName
	if strings.HasPrefix(s, "ansi:") {
		name := s[5:]
		if num, ok := ansiNames[name]; ok {
			return lipgloss.Color(num)
		}
		return lipgloss.Color("7") // fallback white
	}

	// ansi256(n)
	if strings.HasPrefix(s, "ansi256(") && strings.HasSuffix(s, ")") {
		num := s[8 : len(s)-1]
		return lipgloss.Color(num)
	}

	// plain number
	return lipgloss.Color(s)
}

// ResolveAdaptive creates an AdaptiveColor from a light and dark theme color string.
func ResolveAdaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{
		Light: string(Resolve(light)),
		Dark:  string(Resolve(dark)),
	}
}
