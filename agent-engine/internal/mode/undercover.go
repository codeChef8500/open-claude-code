package mode

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/wall-ai/agent-engine/internal/util"
)

// RepoClass classifies a Git repository.
type RepoClass string

const (
	RepoClassInternal RepoClass = "internal"
	RepoClassPublic   RepoClass = "public"
	RepoClassUnknown  RepoClass = "unknown"
)

var (
	repoClassCache     RepoClass
	repoClassCacheOnce sync.Once
)

// IsUndercover reports whether undercover mode should be active.
// Undercover mode is enabled when:
//  1. The USER_TYPE env var equals "ant" (Anthropic employee)
//  2. AND either CLAUDE_CODE_UNDERCOVER=1 is set,
//     OR the current repo is classified as non-internal.
func IsUndercover(cwd string) bool {
	if os.Getenv("USER_TYPE") != "ant" {
		return false
	}
	if util.IsEnvTruthy(os.Getenv("CLAUDE_CODE_UNDERCOVER")) {
		return true
	}
	return ClassifyRepo(context.Background(), cwd) != RepoClassInternal
}

// ClassifyRepo classifies the current Git repository as internal or public.
// Internal repos are those whose remote URL contains "anthropic" in the host.
// The result is cached for the lifetime of the process.
func ClassifyRepo(ctx context.Context, cwd string) RepoClass {
	repoClassCacheOnce.Do(func() {
		repoClassCache = classifyRepo(ctx, cwd)
	})
	return repoClassCache
}

func classifyRepo(ctx context.Context, cwd string) RepoClass {
	remoteURL, err := util.GitGetRemoteURL(ctx, "origin", cwd)
	if err != nil || remoteURL == "" {
		return RepoClassUnknown
	}
	lower := strings.ToLower(remoteURL)
	if strings.Contains(lower, "anthropic") {
		return RepoClassInternal
	}
	return RepoClassPublic
}

// IsUndercoverAllowed reports whether a specific action is permitted even in
// undercover mode (e.g. the allowlist of internal references).
func IsUndercoverAllowed(action string, allowlist []string) bool {
	action = strings.ToLower(action)
	for _, a := range allowlist {
		if strings.ToLower(a) == action {
			return true
		}
	}
	return false
}
