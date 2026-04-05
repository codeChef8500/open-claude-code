package daemon

import (
	"testing"
	"time"
)

func TestParseCronExpression_Valid(t *testing.T) {
	tests := []struct {
		expr string
		want CronFields
	}{
		{
			expr: "* * * * *",
			want: CronFields{
				Minute:     seq(0, 59),
				Hour:       seq(0, 23),
				DayOfMonth: seq(1, 31),
				Month:      seq(1, 12),
				DayOfWeek:  seq(0, 6),
			},
		},
		{
			expr: "0 9 * * 1-5",
			want: CronFields{
				Minute:     []int{0},
				Hour:       []int{9},
				DayOfMonth: seq(1, 31),
				Month:      seq(1, 12),
				DayOfWeek:  []int{1, 2, 3, 4, 5},
			},
		},
		{
			expr: "*/5 * * * *",
			want: CronFields{
				Minute:     []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55},
				Hour:       seq(0, 23),
				DayOfMonth: seq(1, 31),
				Month:      seq(1, 12),
				DayOfWeek:  seq(0, 6),
			},
		},
		{
			expr: "30 14 28 2 *",
			want: CronFields{
				Minute:     []int{30},
				Hour:       []int{14},
				DayOfMonth: []int{28},
				Month:      []int{2},
				DayOfWeek:  seq(0, 6),
			},
		},
		{
			expr: "0,15,30,45 * * * *",
			want: CronFields{
				Minute:     []int{0, 15, 30, 45},
				Hour:       seq(0, 23),
				DayOfMonth: seq(1, 31),
				Month:      seq(1, 12),
				DayOfWeek:  seq(0, 6),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := ParseCronExpression(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertInts(t, "Minute", got.Minute, tt.want.Minute)
			assertInts(t, "Hour", got.Hour, tt.want.Hour)
			assertInts(t, "DayOfMonth", got.DayOfMonth, tt.want.DayOfMonth)
			assertInts(t, "Month", got.Month, tt.want.Month)
			assertInts(t, "DayOfWeek", got.DayOfWeek, tt.want.DayOfWeek)
		})
	}
}

func TestParseCronExpression_DayOfWeek7IsSunday(t *testing.T) {
	// 7 should be treated as Sunday (0)
	got, err := ParseCronExpression("0 0 * * 7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInts(t, "DayOfWeek", got.DayOfWeek, []int{0})

	// Range including 7: 5-7 = Fri,Sat,Sun
	got2, err := ParseCronExpression("0 0 * * 5-7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInts(t, "DayOfWeek", got2.DayOfWeek, []int{0, 5, 6})
}

func TestParseCronExpression_Invalid(t *testing.T) {
	invalids := []string{
		"",
		"* * *",
		"* * * * * *",
		"60 * * * *",
		"-1 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * 32 * *",
		"* * * 0 *",
		"* * * 13 *",
		"* * * * 8",
		"abc * * * *",
	}
	for _, expr := range invalids {
		t.Run(expr, func(t *testing.T) {
			_, err := ParseCronExpression(expr)
			if err == nil {
				t.Fatalf("expected error for %q", expr)
			}
		})
	}
}

func TestParseCronExpression_RangeWithStep(t *testing.T) {
	got, err := ParseCronExpression("1-10/3 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInts(t, "Minute", got.Minute, []int{1, 4, 7, 10})
}

func TestNextRun_Basic(t *testing.T) {
	fields, _ := ParseCronExpression("30 9 * * *")
	from := time.Date(2024, 3, 15, 8, 0, 0, 0, time.Local)
	next := fields.NextRun(from)
	want := time.Date(2024, 3, 15, 9, 30, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("got %v, want %v", next, want)
	}
}

func TestNextRun_StrictlyAfter(t *testing.T) {
	fields, _ := ParseCronExpression("30 9 * * *")
	// from is exactly the fire time — should return next day
	from := time.Date(2024, 3, 15, 9, 30, 0, 0, time.Local)
	next := fields.NextRun(from)
	want := time.Date(2024, 3, 16, 9, 30, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("got %v, want %v", next, want)
	}
}

func TestNextRun_EveryFiveMinutes(t *testing.T) {
	fields, _ := ParseCronExpression("*/5 * * * *")
	from := time.Date(2024, 3, 15, 10, 7, 30, 0, time.Local)
	next := fields.NextRun(from)
	want := time.Date(2024, 3, 15, 10, 10, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("got %v, want %v", next, want)
	}
}

func TestNextRun_DayOfWeekOR(t *testing.T) {
	// When both dom and dow are constrained, OR semantics apply.
	// "0 0 15 * 1" fires on the 15th OR on Mondays
	fields, _ := ParseCronExpression("0 0 15 * 1")

	// 2024-03-14 is Thursday → next Monday is 2024-03-18, next 15th is 2024-03-15
	// OR: 15th comes first
	from := time.Date(2024, 3, 14, 1, 0, 0, 0, time.Local)
	next := fields.NextRun(from)
	want := time.Date(2024, 3, 15, 0, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("got %v, want %v", next, want)
	}
}

func TestNextRun_MonthSkip(t *testing.T) {
	// Only fires in February
	fields, _ := ParseCronExpression("0 0 1 2 *")
	from := time.Date(2024, 3, 1, 0, 0, 0, 0, time.Local)
	next := fields.NextRun(from)
	want := time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)
	if !next.Equal(want) {
		t.Fatalf("got %v, want %v", next, want)
	}
}

func TestNextRunMs(t *testing.T) {
	nowMs := time.Date(2024, 3, 15, 8, 0, 0, 0, time.Local).UnixMilli()
	result := NextRunMs("30 9 * * *", nowMs)
	if result < 0 {
		t.Fatal("expected positive result")
	}
	got := time.UnixMilli(result)
	want := time.Date(2024, 3, 15, 9, 30, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNextRunMs_Invalid(t *testing.T) {
	result := NextRunMs("bad", 0)
	if result != -1 {
		t.Fatalf("expected -1 for invalid cron, got %d", result)
	}
}

func TestCronToHuman(t *testing.T) {
	tests := []struct {
		cron string
		want string
	}{
		{"*/5 * * * *", "Every 5 minutes"},
		{"*/1 * * * *", "Every minute"},
		{"0 * * * *", "Every hour"},
		{"15 * * * *", "Every hour at :15"},
		{"0 */2 * * *", "Every 2 hours"},
		{"0 9 * * 1-5", "Weekdays at 9:00 AM"},
	}
	for _, tt := range tests {
		t.Run(tt.cron, func(t *testing.T) {
			got := CronToHuman(tt.cron)
			if got != tt.want {
				t.Fatalf("CronToHuman(%q) = %q, want %q", tt.cron, got, tt.want)
			}
		})
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func seq(lo, hi int) []int {
	out := make([]int, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		out = append(out, i)
	}
	return out
}

func assertInts(t *testing.T, name string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len %d != %d\ngot:  %v\nwant: %v", name, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s[%d]: %d != %d\ngot:  %v\nwant: %v", name, i, got[i], want[i], got, want)
		}
	}
}
