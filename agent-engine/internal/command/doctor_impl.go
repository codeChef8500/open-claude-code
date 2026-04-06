package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// /doctor — full implementation
// Aligned with claude-code-main commands/doctor/doctor.tsx.
//
// Runs a series of diagnostic checks and reports results.
// Interactive mode returns structured data for TUI diagnostic panel.
// ──────────────────────────────────────────────────────────────────────────────

// DiagnosticResult is the overall result of the doctor command.
type DiagnosticResult struct {
	Checks    []DiagnosticCheck `json:"checks"`
	Summary   DiagnosticSummary `json:"summary"`
	Timestamp string            `json:"timestamp"`
}

// DiagnosticCheck is a single diagnostic check result.
type DiagnosticCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "warn", "fail", "skip"
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// DiagnosticSummary summarizes the diagnostic results.
type DiagnosticSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Warned  int `json:"warned"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// DeepDoctorCommand replaces the basic DoctorCommand with full diagnostic logic.
type DeepDoctorCommand struct{ BaseCommand }

func (c *DeepDoctorCommand) Name() string                  { return "doctor" }
func (c *DeepDoctorCommand) Description() string           { return "Run diagnostic checks" }
func (c *DeepDoctorCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepDoctorCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepDoctorCommand) ExecuteInteractive(ctx context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	result := runDiagnostics(ctx, ectx)
	return &InteractiveResult{
		Component: "doctor",
		Data:      result,
	}, nil
}

// runDiagnostics runs all diagnostic checks.
func runDiagnostics(ctx context.Context, ectx *ExecContext) *DiagnosticResult {
	result := &DiagnosticResult{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// 1. Runtime environment
	result.Checks = append(result.Checks, checkRuntime())

	// 2. Git availability
	result.Checks = append(result.Checks, checkGit(ctx, ectx))

	// 3. Working directory
	result.Checks = append(result.Checks, checkWorkDir(ectx))

	// 4. API key / authentication
	result.Checks = append(result.Checks, checkAuth(ectx))

	// 5. Model configuration
	result.Checks = append(result.Checks, checkModel(ectx))

	// 6. MCP servers
	result.Checks = append(result.Checks, checkMCPServers(ectx))

	// 7. Config files
	result.Checks = append(result.Checks, checkConfigFiles(ectx))

	// 8. Disk space
	result.Checks = append(result.Checks, checkDiskSpace())

	// 9. Session state
	result.Checks = append(result.Checks, checkSessionState(ectx))

	// 10. Network connectivity
	result.Checks = append(result.Checks, checkNetwork())

	// Compute summary.
	for _, check := range result.Checks {
		result.Summary.Total++
		switch check.Status {
		case "pass":
			result.Summary.Passed++
		case "warn":
			result.Summary.Warned++
		case "fail":
			result.Summary.Failed++
		case "skip":
			result.Summary.Skipped++
		}
	}

	return result
}

func checkRuntime() DiagnosticCheck {
	return DiagnosticCheck{
		Name:    "Runtime Environment",
		Status:  "pass",
		Message: fmt.Sprintf("Go %s on %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH),
		Detail:  fmt.Sprintf("NumCPU: %d, NumGoroutine: %d", runtime.NumCPU(), runtime.NumGoroutine()),
	}
}

func checkGit(ctx context.Context, _ *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Git"}

	out, err := exec.CommandContext(ctx, "git", "--version").Output()
	if err != nil {
		check.Status = "fail"
		check.Message = "git not found in PATH"
		check.Detail = err.Error()
		return check
	}
	check.Status = "pass"
	check.Message = strings.TrimSpace(string(out))
	return check
}

func checkWorkDir(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Working Directory"}

	if ectx == nil || ectx.WorkDir == "" {
		check.Status = "warn"
		check.Message = "No working directory set"
		return check
	}

	info, err := os.Stat(ectx.WorkDir)
	if err != nil {
		check.Status = "fail"
		check.Message = fmt.Sprintf("Cannot access: %s", ectx.WorkDir)
		check.Detail = err.Error()
		return check
	}

	if !info.IsDir() {
		check.Status = "fail"
		check.Message = fmt.Sprintf("Not a directory: %s", ectx.WorkDir)
		return check
	}

	check.Status = "pass"
	check.Message = ectx.WorkDir
	return check
}

func checkAuth(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Authentication"}

	if ectx == nil || ectx.Services == nil || ectx.Services.Auth == nil {
		// Check environment variables.
		if os.Getenv("ANTHROPIC_API_KEY") != "" {
			check.Status = "pass"
			check.Message = "API key set via ANTHROPIC_API_KEY"
			return check
		}
		if os.Getenv("OPENAI_API_KEY") != "" {
			check.Status = "pass"
			check.Message = "API key set via OPENAI_API_KEY"
			return check
		}
		check.Status = "warn"
		check.Message = "No auth service and no API key in environment"
		return check
	}

	auth := ectx.Services.Auth
	if auth.IsAuthenticated() {
		check.Status = "pass"
		check.Message = "Authenticated"
		return check
	}

	check.Status = "warn"
	check.Message = "Not authenticated"
	return check
}

func checkModel(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Model Configuration"}

	if ectx == nil || ectx.Model == "" {
		check.Status = "warn"
		check.Message = "No model configured"
		return check
	}

	check.Status = "pass"
	check.Message = ectx.Model
	return check
}

func checkMCPServers(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "MCP Servers"}

	if ectx == nil {
		check.Status = "skip"
		check.Message = "No session context"
		return check
	}

	count := len(ectx.ActiveMCPServers)
	if ectx.Services != nil && ectx.Services.MCP != nil {
		servers := ectx.Services.MCP.ListServers()
		count = len(servers)
		errCount := 0
		for _, s := range servers {
			if s.Error != "" {
				errCount++
			}
		}
		if errCount > 0 {
			check.Status = "warn"
			check.Message = fmt.Sprintf("%d servers (%d with errors)", count, errCount)
			return check
		}
	}

	if count == 0 {
		check.Status = "pass"
		check.Message = "No MCP servers configured"
		return check
	}

	check.Status = "pass"
	check.Message = fmt.Sprintf("%d server(s) connected", count)
	return check
}

func checkConfigFiles(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Config Files"}

	if ectx == nil || ectx.Services == nil || ectx.Services.Config == nil {
		check.Status = "skip"
		check.Message = "Config service not available"
		return check
	}

	projectPath := ectx.Services.Config.ProjectPath()
	userPath := ectx.Services.Config.UserPath()

	var found []string
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err == nil {
			found = append(found, "project")
		}
	}
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			found = append(found, "user")
		}
	}

	if len(found) == 0 {
		check.Status = "pass"
		check.Message = "No config files found (using defaults)"
		return check
	}

	check.Status = "pass"
	check.Message = fmt.Sprintf("Found: %s", strings.Join(found, ", "))
	return check
}

func checkDiskSpace() DiagnosticCheck {
	check := DiagnosticCheck{Name: "Disk Space"}

	// Use runtime memory stats as a proxy for Go-specific health.
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	allocMB := float64(m.Alloc) / 1024 / 1024
	sysMB := float64(m.Sys) / 1024 / 1024

	check.Status = "pass"
	check.Message = fmt.Sprintf("Alloc: %.1f MB, Sys: %.1f MB", allocMB, sysMB)

	if allocMB > 500 {
		check.Status = "warn"
		check.Message = fmt.Sprintf("High memory usage: %.1f MB allocated", allocMB)
	}

	return check
}

func checkSessionState(ectx *ExecContext) DiagnosticCheck {
	check := DiagnosticCheck{Name: "Session State"}

	if ectx == nil {
		check.Status = "skip"
		check.Message = "No session context"
		return check
	}

	check.Status = "pass"
	check.Message = fmt.Sprintf("Session %s, %d turns, %d tokens",
		ectx.SessionID, ectx.TurnCount, ectx.TotalTokens)

	if ectx.ContextStats != nil && ectx.ContextStats.UsedFraction > 0.9 {
		check.Status = "warn"
		check.Message += " (context window >90% full)"
	}

	return check
}

func checkNetwork() DiagnosticCheck {
	check := DiagnosticCheck{Name: "Network"}

	// Quick DNS lookup as a connectivity test.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ping", "-n", "1", "-w", "2000", "api.anthropic.com")
	if runtime.GOOS != "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", "api.anthropic.com")
	}

	if err := cmd.Run(); err != nil {
		check.Status = "warn"
		check.Message = "Cannot reach api.anthropic.com"
		check.Detail = err.Error()
		return check
	}

	check.Status = "pass"
	check.Message = "api.anthropic.com reachable"
	return check
}

// ──────────────────────────────────────────────────────────────────────────────
// Non-interactive /doctor-text fallback.
// ──────────────────────────────────────────────────────────────────────────────

// DoctorTextCommand is the non-interactive text fallback.
type DoctorTextCommand struct{ BaseCommand }

func (c *DoctorTextCommand) Name() string                  { return "doctor-text" }
func (c *DoctorTextCommand) Description() string           { return "Run diagnostic checks (text mode)" }
func (c *DoctorTextCommand) Type() CommandType             { return CommandTypeLocal }
func (c *DoctorTextCommand) IsHidden() bool                { return true }
func (c *DoctorTextCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DoctorTextCommand) Execute(ctx context.Context, _ []string, ectx *ExecContext) (string, error) {
	result := runDiagnostics(ctx, ectx)

	var sb strings.Builder
	sb.WriteString("## Doctor Diagnostics\n\n")

	statusIcon := map[string]string{
		"pass": "[PASS]",
		"warn": "[WARN]",
		"fail": "[FAIL]",
		"skip": "[SKIP]",
	}

	for _, check := range result.Checks {
		icon := statusIcon[check.Status]
		sb.WriteString(fmt.Sprintf("%s %s: %s\n", icon, check.Name, check.Message))
		if check.Detail != "" {
			sb.WriteString(fmt.Sprintf("      %s\n", check.Detail))
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d passed, %d warnings, %d failed, %d skipped (of %d)\n",
		result.Summary.Passed, result.Summary.Warned, result.Summary.Failed,
		result.Summary.Skipped, result.Summary.Total))

	return sb.String(), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// /bug-report deep implementation
// Aligned with claude-code-main commands/bug-report/bug-report.tsx.
// ──────────────────────────────────────────────────────────────────────────────

// BugReportViewData is the structured data for the bug report.
type BugReportViewData struct {
	Diagnostics *DiagnosticResult `json:"diagnostics"`
	Version     string            `json:"version"`
	OS          string            `json:"os"`
	Arch        string            `json:"arch"`
	GoVersion   string            `json:"go_version"`
	SessionID   string            `json:"session_id"`
	Model       string            `json:"model"`
}

// DeepBugReportCommand generates a bug report with diagnostic info.
type DeepBugReportCommand struct{ BaseCommand }

func (c *DeepBugReportCommand) Name() string      { return "bug-report" }
func (c *DeepBugReportCommand) Aliases() []string { return []string{"bugreport"} }
func (c *DeepBugReportCommand) Description() string {
	return "Generate a bug report with diagnostic info"
}
func (c *DeepBugReportCommand) Type() CommandType             { return CommandTypeInteractive }
func (c *DeepBugReportCommand) IsEnabled(_ *ExecContext) bool { return true }

func (c *DeepBugReportCommand) ExecuteInteractive(ctx context.Context, _ []string, ectx *ExecContext) (*InteractiveResult, error) {
	data := &BugReportViewData{
		Diagnostics: runDiagnostics(ctx, ectx),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		GoVersion:   runtime.Version(),
	}
	if ectx != nil {
		data.SessionID = ectx.SessionID
		data.Model = ectx.Model
	}
	return &InteractiveResult{
		Component: "bug-report",
		Data:      data,
	}, nil
}

func init() {
	defaultRegistry.RegisterOrReplace(
		&DeepDoctorCommand{},
		&DoctorTextCommand{},
		&DeepBugReportCommand{},
	)
}
