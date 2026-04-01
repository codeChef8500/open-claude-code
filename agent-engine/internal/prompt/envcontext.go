package prompt

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// EnvContext holds the dynamic environment details injected into the system prompt.
type EnvContext struct {
	Platform     string
	Arch         string
	WorkDir      string
	CurrentTime  string
	Shell        string
	HomeDir      string
	IsGitRepo    bool
	GitBranch    string
}

// BuildEnvContext collects the current runtime environment.
func BuildEnvContext(workDir string) *EnvContext {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	if shell == "" && runtime.GOOS == "windows" {
		shell = "cmd.exe"
	}

	return &EnvContext{
		Platform:    runtime.GOOS,
		Arch:        runtime.GOARCH,
		WorkDir:     workDir,
		CurrentTime: time.Now().Format(time.RFC3339),
		Shell:       shell,
		HomeDir:     home,
	}
}

// Render formats the environment context as a system prompt section.
func (e *EnvContext) Render() string {
	s := fmt.Sprintf(`<env>
platform: %s/%s
cwd: %s
time: %s
shell: %s
home: %s`,
		e.Platform, e.Arch,
		e.WorkDir,
		e.CurrentTime,
		e.Shell,
		e.HomeDir,
	)
	if e.IsGitRepo && e.GitBranch != "" {
		s += fmt.Sprintf("\ngit_branch: %s", e.GitBranch)
	}
	s += "\n</env>"
	return s
}
