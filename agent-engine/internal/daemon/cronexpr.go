package daemon

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronFields holds the expanded set of matching values for each cron field.
// Ported from claude-code-main utils/cron.ts.
type CronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int // 0=Sunday, 1=Monday, ..., 6=Saturday
}

type fieldRange struct {
	min int
	max int
}

var fieldRanges = []fieldRange{
	{min: 0, max: 59}, // minute
	{min: 0, max: 23}, // hour
	{min: 1, max: 31}, // dayOfMonth
	{min: 1, max: 12}, // month
	{min: 0, max: 6},  // dayOfWeek (0=Sunday; 7 accepted as Sunday alias)
}

// expandField parses a single cron field into a sorted slice of matching values.
// Supports: wildcard (*), step (*/N), range (N-M), range with step (N-M/S),
// comma-separated lists, and plain integers.
func expandField(field string, fr fieldRange) ([]int, error) {
	set := make(map[int]struct{})

	isDow := fr.min == 0 && fr.max == 6

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// wildcard or */N
		if strings.HasPrefix(part, "*") {
			rest := part[1:]
			step := 1
			if strings.HasPrefix(rest, "/") {
				s, err := strconv.Atoi(rest[1:])
				if err != nil || s < 1 {
					return nil, fmt.Errorf("invalid step in %q", part)
				}
				step = s
			} else if rest != "" {
				return nil, fmt.Errorf("invalid field %q", part)
			}
			for i := fr.min; i <= fr.max; i += step {
				set[i] = struct{}{}
			}
			continue
		}

		// N-M or N-M/S
		if dashIdx := strings.Index(part, "-"); dashIdx > 0 {
			rangeAndStep := part[dashIdx+1:]
			loStr := part[:dashIdx]
			hiStr := rangeAndStep
			step := 1
			if slashIdx := strings.Index(rangeAndStep, "/"); slashIdx >= 0 {
				hiStr = rangeAndStep[:slashIdx]
				s, err := strconv.Atoi(rangeAndStep[slashIdx+1:])
				if err != nil || s < 1 {
					return nil, fmt.Errorf("invalid step in range %q", part)
				}
				step = s
			}
			lo, err1 := strconv.Atoi(loStr)
			hi, err2 := strconv.Atoi(hiStr)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			effMax := fr.max
			if isDow {
				effMax = 7 // accept 7 as Sunday alias
			}
			if lo > hi || lo < fr.min || hi > effMax {
				return nil, fmt.Errorf("range out of bounds in %q", part)
			}
			for i := lo; i <= hi; i += step {
				v := i
				if isDow && v == 7 {
					v = 0
				}
				set[v] = struct{}{}
			}
			continue
		}

		// plain N
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		if isDow && n == 7 {
			n = 0
		}
		if n < fr.min || n > fr.max {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", n, fr.min, fr.max)
		}
		set[n] = struct{}{}
	}

	if len(set) == 0 {
		return nil, fmt.Errorf("empty field")
	}

	result := make([]int, 0, len(set))
	for v := range set {
		result = append(result, v)
	}
	sortInts(result)
	return result, nil
}

// sortInts sorts a small int slice in ascending order (insertion sort).
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}

// ParseCronExpression parses a 5-field cron expression into expanded CronFields.
// Returns an error if the expression is invalid or uses unsupported syntax.
func ParseCronExpression(expr string) (*CronFields, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	expanded := make([][]int, 5)
	for i := 0; i < 5; i++ {
		vals, err := expandField(parts[i], fieldRanges[i])
		if err != nil {
			return nil, fmt.Errorf("field %d (%q): %w", i+1, parts[i], err)
		}
		expanded[i] = vals
	}

	return &CronFields{
		Minute:     expanded[0],
		Hour:       expanded[1],
		DayOfMonth: expanded[2],
		Month:      expanded[3],
		DayOfWeek:  expanded[4],
	}, nil
}

// NextRun computes the next time strictly after `from` that matches the cron
// fields, using local timezone. Bounded at 366 days; returns zero time if no
// match (should not happen for valid cron expressions).
//
// Standard cron semantics: when both dayOfMonth and dayOfWeek are constrained
// (neither is the full range), a date matches if EITHER matches.
//
// DST: fixed-hour crons targeting a spring-forward gap skip that day; wildcard-
// hour crons fire at the first valid minute after the gap. This matches
// vixie-cron behavior.
func (f *CronFields) NextRun(from time.Time) time.Time {
	minuteSet := toSet(f.Minute)
	hourSet := toSet(f.Hour)
	domSet := toSet(f.DayOfMonth)
	monthSet := toSet(f.Month)
	dowSet := toSet(f.DayOfWeek)

	domWild := len(f.DayOfMonth) == 31
	dowWild := len(f.DayOfWeek) == 7

	// Round up to the next whole minute (strictly after `from`)
	t := from.Truncate(time.Minute).Add(time.Minute)

	maxIter := 366 * 24 * 60
	for i := 0; i < maxIter; i++ {
		month := int(t.Month())
		if !monthSet[month] {
			// Jump to start of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		dom := t.Day()
		dow := int(t.Weekday())
		var dayMatches bool
		if domWild && dowWild {
			dayMatches = true
		} else if domWild {
			dayMatches = dowSet[dow]
		} else if dowWild {
			dayMatches = domSet[dom]
		} else {
			dayMatches = domSet[dom] || dowSet[dow]
		}

		if !dayMatches {
			// Jump to start of next day
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !hourSet[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}

		if !minuteSet[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}

// NextRunMs returns the next run time as epoch milliseconds, or -1 if none
// found within 366 days.
func NextRunMs(cronExpr string, nowMs int64) int64 {
	fields, err := ParseCronExpression(cronExpr)
	if err != nil {
		return -1
	}
	from := time.UnixMilli(nowMs)
	next := fields.NextRun(from)
	if next.IsZero() {
		return -1
	}
	return next.UnixMilli()
}

func toSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// ─── cronToHuman ────────────────────────────────────────────────────────────

var dayNames = []string{
	"Sunday", "Monday", "Tuesday", "Wednesday",
	"Thursday", "Friday", "Saturday",
}

func formatLocalTime(minute, hour int) string {
	t := time.Date(2000, 1, 1, hour, minute, 0, 0, time.Local)
	h := t.Hour()
	m := t.Minute()
	period := "AM"
	if h >= 12 {
		period = "PM"
	}
	displayH := h % 12
	if displayH == 0 {
		displayH = 12
	}
	return fmt.Sprintf("%d:%02d %s", displayH, m, period)
}

// CronToHuman converts a 5-field cron expression to a human-readable description.
// Covers common patterns; falls through to the raw cron string for anything else.
func CronToHuman(cron string) string {
	parts := strings.Fields(strings.TrimSpace(cron))
	if len(parts) != 5 {
		return cron
	}

	minute, hour, dayOfMonth, month, dayOfWeek := parts[0], parts[1], parts[2], parts[3], parts[4]

	// Every N minutes: */N * * * *
	if strings.HasPrefix(minute, "*/") && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		n, err := strconv.Atoi(minute[2:])
		if err == nil {
			if n == 1 {
				return "Every minute"
			}
			return fmt.Sprintf("Every %d minutes", n)
		}
	}

	// Every hour: N * * * *
	if isDigits(minute) && hour == "*" && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		m, _ := strconv.Atoi(minute)
		if m == 0 {
			return "Every hour"
		}
		return fmt.Sprintf("Every hour at :%02d", m)
	}

	// Every N hours: N */H * * *
	if isDigits(minute) && strings.HasPrefix(hour, "*/") && dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		n, err := strconv.Atoi(hour[2:])
		m, _ := strconv.Atoi(minute)
		if err == nil {
			suffix := ""
			if m != 0 {
				suffix = fmt.Sprintf(" at :%02d", m)
			}
			if n == 1 {
				return "Every hour" + suffix
			}
			return fmt.Sprintf("Every %d hours%s", n, suffix)
		}
	}

	// Remaining cases need numeric hour+minute
	if !isDigits(minute) || !isDigits(hour) {
		return cron
	}
	m, _ := strconv.Atoi(minute)
	h, _ := strconv.Atoi(hour)

	// Daily at specific time: M H * * *
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "*" {
		return fmt.Sprintf("Every day at %s", formatLocalTime(m, h))
	}

	// Specific day of week: M H * * D
	if dayOfMonth == "*" && month == "*" && len(dayOfWeek) == 1 && isDigits(dayOfWeek) {
		dayIdx, _ := strconv.Atoi(dayOfWeek)
		dayIdx = dayIdx % 7
		if dayIdx >= 0 && dayIdx < len(dayNames) {
			return fmt.Sprintf("Every %s at %s", dayNames[dayIdx], formatLocalTime(m, h))
		}
	}

	// Weekdays: M H * * 1-5
	if dayOfMonth == "*" && month == "*" && dayOfWeek == "1-5" {
		return fmt.Sprintf("Weekdays at %s", formatLocalTime(m, h))
	}

	return cron
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
