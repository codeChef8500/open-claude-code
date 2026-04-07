package spinnerv2

import (
	"fmt"
	"math/rand"
	"time"
)

// CompletionVerbsEN is the list of past-tense verbs used when a turn completes,
// matching claude-code-main's turnCompletionVerbs.ts exactly.
var CompletionVerbsEN = []string{
	"Baked",
	"Brewed",
	"Churned",
	"Cogitated",
	"Cooked",
	"Crunched",
	"Sautéed",
	"Worked",
}

// CompletionVerbsCN is the Chinese list of completion phrases.
var CompletionVerbsCN = []string{
	"炖好了",
	"烤好了",
	"酿好了",
	"炼成了",
	"搞定了",
	"完成了",
	"出炉了",
	"炒好了",
	"煮好了",
	"整完了",
	"搞掂了",
	"妙手得之",
	"大功告成",
	"功德圆满",
	"天降神迹",
	"水到渠成",
}

// RandomCompletionVerb returns a random completion verb in the current language.
func RandomCompletionVerb() string {
	switch currentLang {
	case LangEN:
		return CompletionVerbsEN[rand.Intn(len(CompletionVerbsEN))]
	default: // LangCN
		return CompletionVerbsCN[rand.Intn(len(CompletionVerbsCN))]
	}
}

// FormatTurnCompletion returns a turn-completion message like "Worked for 5s" (EN)
// or "炖好了 · 5s" (CN).
func FormatTurnCompletion(elapsed time.Duration) string {
	verb := RandomCompletionVerb()
	d := elapsed.Truncate(time.Second)
	if d < time.Second {
		d = time.Second
	}
	if currentLang == LangCN {
		if d < time.Minute {
			return fmt.Sprintf("%s · %ds", verb, int(d.Seconds()))
		}
		m := int(d.Minutes())
		s := int(d.Seconds()) - m*60
		if s == 0 {
			return fmt.Sprintf("%s · %dm", verb, m)
		}
		return fmt.Sprintf("%s · %dm%ds", verb, m, s)
	}
	if d < time.Minute {
		return fmt.Sprintf("%s for %ds", verb, int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) - m*60
	if s == 0 {
		return fmt.Sprintf("%s for %dm", verb, m)
	}
	return fmt.Sprintf("%s for %dm %ds", verb, m, s)
}
