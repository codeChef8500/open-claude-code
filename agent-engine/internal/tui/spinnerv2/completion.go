package spinnerv2

import (
	"fmt"
	"math/rand"
	"time"
)

// CompletionVerbs is the list of past-tense verbs used when a turn completes,
// matching claude-code-main's turnCompletionVerbs.ts exactly.
var CompletionVerbs = []string{
	"Baked",
	"Brewed",
	"Churned",
	"Cogitated",
	"Cooked",
	"Crunched",
	"Sautéed",
	"Worked",
}

// RandomCompletionVerb returns a random past-tense verb.
func RandomCompletionVerb() string {
	return CompletionVerbs[rand.Intn(len(CompletionVerbs))]
}

// FormatTurnCompletion returns a turn-completion message like "Worked for 5s".
func FormatTurnCompletion(elapsed time.Duration) string {
	verb := RandomCompletionVerb()
	d := elapsed.Truncate(time.Second)
	if d < time.Second {
		d = time.Second
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
