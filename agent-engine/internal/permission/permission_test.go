package permission

import (
	"context"
	"testing"
)

// ── DenialTrackingState tests ───────────────────────────────────────────────

func TestDenialTracking_RecordDenial(t *testing.T) {
	s := NewDenialTrackingState()
	s = s.RecordDenial()
	if s.ConsecutiveDenials != 1 {
		t.Errorf("consecutive: expected 1, got %d", s.ConsecutiveDenials)
	}
	if s.TotalDenials != 1 {
		t.Errorf("total: expected 1, got %d", s.TotalDenials)
	}
}

func TestDenialTracking_RecordSuccess(t *testing.T) {
	s := NewDenialTrackingState()
	s = s.RecordDenial().RecordDenial()
	s = s.RecordSuccess()
	if s.ConsecutiveDenials != 0 {
		t.Errorf("consecutive should reset to 0, got %d", s.ConsecutiveDenials)
	}
	if s.TotalDenials != 2 {
		t.Errorf("total should remain 2, got %d", s.TotalDenials)
	}
}

func TestDenialTracking_SuccessNoOp(t *testing.T) {
	s := NewDenialTrackingState()
	s2 := s.RecordSuccess()
	if s2.ConsecutiveDenials != 0 || s2.TotalDenials != 0 {
		t.Error("RecordSuccess on fresh state should be no-op")
	}
}

func TestDenialTracking_ShouldFallback_Consecutive(t *testing.T) {
	s := NewDenialTrackingState()
	for i := 0; i < DenialLimits.MaxConsecutive; i++ {
		s = s.RecordDenial()
	}
	if !s.ShouldFallbackToPrompting() {
		t.Error("should fallback after max consecutive denials")
	}
}

func TestDenialTracking_ShouldFallback_Total(t *testing.T) {
	s := NewDenialTrackingState()
	for i := 0; i < DenialLimits.MaxTotal; i++ {
		s = s.RecordDenial()
		if i < DenialLimits.MaxConsecutive-1 {
			s = s.RecordSuccess() // reset consecutive but keep total growing
		}
	}
	if !s.ShouldFallbackToPrompting() {
		t.Error("should fallback after max total denials")
	}
}

func TestDenialTracking_NoFallbackBelowThreshold(t *testing.T) {
	s := NewDenialTrackingState()
	s = s.RecordDenial()
	if s.ShouldFallbackToPrompting() {
		t.Error("should not fallback after 1 denial")
	}
}

// ── NoopClassifier tests ────────────────────────────────────────────────────

func TestNoopClassifier_AlwaysAllows(t *testing.T) {
	c := &NoopClassifier{}
	result, err := c.Classify(context.Background(), ClassifyRequest{
		ToolName: "Bash",
		Action:   "rm -rf /tmp/safe",
	})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if result.ShouldBlock {
		t.Error("NoopClassifier should never block")
	}
	if result.Stage != "noop" {
		t.Errorf("expected stage 'noop', got %q", result.Stage)
	}
}

// ── ToolPermissionContext tests ──────────────────────────────────────────────

func TestToolPermissionContext_AllRules(t *testing.T) {
	ctx := &ToolPermissionContext{
		RulesBySource: map[RuleSource][]Rule{
			RuleSourceUserSettings: {
				{Type: RuleAllow, Pattern: "*.go", ToolName: "Read"},
			},
			RuleSourceProjectSettings: {
				{Type: RuleDeny, Pattern: "/etc/*", ToolName: "Bash"},
			},
		},
		SessionRules: []Rule{
			{Type: RuleAllow, Pattern: "/tmp/*", ToolName: "Write"},
		},
	}

	all := ctx.AllRules()
	if len(all) != 3 {
		t.Errorf("expected 3 rules, got %d", len(all))
	}
}

func TestToolPermissionContext_AllRules_Empty(t *testing.T) {
	ctx := &ToolPermissionContext{}
	all := ctx.AllRules()
	if len(all) != 0 {
		t.Errorf("expected 0 rules, got %d", len(all))
	}
}

// ── Mode / Result / Behavior constants ──────────────────────────────────────

func TestModeConstants(t *testing.T) {
	modes := ExternalPermissionModes
	if len(modes) != 6 {
		t.Errorf("expected 6 external modes, got %d", len(modes))
	}
	found := false
	for _, m := range modes {
		if m == ModeAutoApprove {
			found = true
		}
	}
	if !found {
		t.Error("ModeAutoApprove not in external modes")
	}
}

func TestResultConstants(t *testing.T) {
	if ResultAllow != 0 {
		t.Errorf("expected ResultAllow=0, got %d", ResultAllow)
	}
	if ResultDeny != 1 {
		t.Errorf("expected ResultDeny=1, got %d", ResultDeny)
	}
	if ResultAsk != 2 {
		t.Errorf("expected ResultAsk=2, got %d", ResultAsk)
	}
}

func TestDangerousShellPatterns(t *testing.T) {
	if len(DangerousShellPatterns) == 0 {
		t.Error("DangerousShellPatterns should not be empty")
	}
	// Check known entries.
	found := false
	for _, p := range DangerousShellPatterns {
		if p == "rm -rf /" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'rm -rf /' in DangerousShellPatterns")
	}
}
