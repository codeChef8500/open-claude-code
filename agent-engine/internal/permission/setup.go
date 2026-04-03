package permission

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// SetupConfig holds everything needed to initialize the permission system.
type SetupConfig struct {
	// Mode is the requested permission mode (from CLI flag, env, or settings).
	Mode Mode
	// WorkDir is the current working directory.
	WorkDir string
	// AdditionalDirs are extra directories that should be writable.
	AdditionalDirs []string
	// ProjectSettingsPath is the path to the project settings.json file.
	ProjectSettingsPath string
	// UserSettingsPath is the path to the user-level settings.json file.
	UserSettingsPath string
	// PermissionStorePath is the path to the persisted permissions file.
	PermissionStorePath string
	// DisableAudit disables audit logging.
	DisableAudit bool
	// AuditLogPath overrides the default audit log path.
	AuditLogPath string
	// AutoModeRules are the user-configured auto-mode rules.
	AutoModeRules *AutoModeRules
	// LLMClassifier is the optional LLM-backed classifier for auto-mode.
	LLMClassifier Classifier
	// AskFunc is the callback for user confirmation prompts.
	AskFunc func(ctx context.Context, tool, desc string) (bool, error)
}

// SetupResult contains the initialized permission components.
type SetupResult struct {
	Checker       *Checker
	Store         *PermissionStore
	Audit         AuditRecorder
	FSChecker     *FilesystemPermissionChecker
	PathValidator *PathValidator
}

// Setup initializes the permission system from configuration.
// This is the main entry point for permission system initialization.
func Setup(cfg SetupConfig) (*SetupResult, error) {
	result := &SetupResult{}

	// 1. Resolve permission mode from environment or config.
	mode := resolveMode(cfg.Mode)

	// 2. Load persisted permission store.
	storePath := cfg.PermissionStorePath
	if storePath == "" {
		storePath = DefaultPermissionStorePath(cfg.WorkDir)
	}
	store := NewPermissionStore(storePath)
	if err := store.Load(); err != nil {
		// Non-fatal: start with empty store.
		store = NewPermissionStore(storePath)
	}
	result.Store = store

	// 3. Build allow/deny rules from store.
	allow, deny := store.ToRules()

	// 4. Merge global permission store rules (if exists).
	globalPath := GlobalPermissionStorePath()
	if globalPath != "" {
		globalStore := NewPermissionStore(globalPath)
		if err := globalStore.Load(); err == nil {
			gAllow, gDeny := globalStore.ToRules()
			allow = append(gAllow, allow...)
			deny = append(gDeny, deny...)
		}
	}

	// 5. Build the checker.
	allowedDirs := append([]string{cfg.WorkDir}, cfg.AdditionalDirs...)
	checker := NewChecker(mode, allow, deny, allowedDirs, nil)

	// 6. Set up auto-mode classifier if in auto mode.
	if mode == ModeAutoApprove {
		rules := AutoModeRules{}
		if cfg.AutoModeRules != nil {
			rules = *cfg.AutoModeRules
		}
		classifier := NewYoloClassifier(rules, cfg.WorkDir, cfg.LLMClassifier)
		checker.SetClassifier(classifier)
	}

	// 7. Set up ask function.
	if cfg.AskFunc != nil {
		checker.SetAskFunc(cfg.AskFunc)
	}

	result.Checker = checker

	// 8. Set up audit log.
	if !cfg.DisableAudit {
		auditPath := cfg.AuditLogPath
		if auditPath == "" {
			auditPath = defaultAuditLogPath(cfg.WorkDir)
		}
		audit, err := NewAuditLog(auditPath)
		if err != nil {
			result.Audit = NopAuditLog{}
		} else {
			result.Audit = audit
		}
	} else {
		result.Audit = NopAuditLog{}
	}

	// 9. Set up filesystem permission checker.
	result.FSChecker = &FilesystemPermissionChecker{
		PermittedDirs: allowedDirs,
		WorkDir:       cfg.WorkDir,
	}

	// 10. Set up path validator.
	result.PathValidator = NewPathValidator(cfg.WorkDir, cfg.AdditionalDirs)

	return result, nil
}

// resolveMode determines the effective permission mode from configuration,
// environment variables, and defaults.
func resolveMode(requested Mode) Mode {
	// Environment variable override.
	envMode := os.Getenv("CLAUDE_PERMISSION_MODE")
	if envMode != "" {
		switch strings.ToLower(envMode) {
		case "auto":
			return ModeAutoApprove
		case "bypass":
			return ModeBypassAll
		case "plan":
			return ModePlan
		case "accept_edits", "acceptedits":
			return ModeAcceptEdits
		case "dont_ask", "dontask":
			return ModeDontAsk
		default:
			return ModeDefault
		}
	}

	if requested != "" {
		return requested
	}
	return ModeDefault
}

// defaultAuditLogPath returns the default path for the permission audit log.
func defaultAuditLogPath(workDir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return fmt.Sprintf("%s/.claude/permission-audit.jsonl", home)
}

// ValidateMode checks if a mode string is a valid permission mode.
func ValidateMode(mode string) (Mode, error) {
	m := Mode(mode)
	for _, valid := range ExternalPermissionModes {
		if m == valid {
			return m, nil
		}
	}
	return "", fmt.Errorf("invalid permission mode: %q (valid: %v)", mode, ExternalPermissionModes)
}

// DescribeMode returns a human-readable description of a permission mode.
func DescribeMode(mode Mode) string {
	switch mode {
	case ModeDefault:
		return "Default — ask for sensitive operations"
	case ModeAutoApprove:
		return "Auto — LLM classifier evaluates tool safety"
	case ModeBypassAll:
		return "Bypass — all operations permitted (dangerous)"
	case ModePlan:
		return "Plan — read-only mode, no write operations"
	case ModeAcceptEdits:
		return "Accept Edits — file edits auto-approved"
	case ModeDontAsk:
		return "Don't Ask — deny anything not auto-allowed"
	case ModeBubble:
		return "Bubble — delegate to parent agent"
	default:
		return fmt.Sprintf("Unknown mode: %s", mode)
	}
}
