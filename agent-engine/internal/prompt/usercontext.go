package prompt

import "fmt"

// UserContext holds per-user contextual metadata injected into conversations.
type UserContext struct {
	Username   string
	WorkDir    string
	CustomInfo map[string]string
}

// InjectUserContext wraps userCtx in <user_context>…</user_context> XML tags
// and prepends it to the first user message (mirrors TS implementation).
func InjectUserContext(uctx *UserContext) string {
	if uctx == nil {
		return ""
	}
	parts := ""
	if uctx.Username != "" {
		parts += fmt.Sprintf("  <username>%s</username>\n", uctx.Username)
	}
	if uctx.WorkDir != "" {
		parts += fmt.Sprintf("  <working_directory>%s</working_directory>\n", uctx.WorkDir)
	}
	for k, v := range uctx.CustomInfo {
		parts += fmt.Sprintf("  <%s>%s</%s>\n", k, v, k)
	}
	if parts == "" {
		return ""
	}
	return fmt.Sprintf("<user_context>\n%s</user_context>", parts)
}
