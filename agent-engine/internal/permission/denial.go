package permission

// DenialTrackingState tracks consecutive and total denials for a permission
// classifier, determining when to fall back to interactive prompting.
// Aligned with claude-code-main's denialTracking.ts.
type DenialTrackingState struct {
	ConsecutiveDenials int `json:"consecutive_denials"`
	TotalDenials       int `json:"total_denials"`
}

// DenialLimits defines thresholds for falling back to prompting.
var DenialLimits = struct {
	MaxConsecutive int
	MaxTotal       int
}{
	MaxConsecutive: 3,
	MaxTotal:       20,
}

// NewDenialTrackingState creates a fresh denial tracking state.
func NewDenialTrackingState() DenialTrackingState {
	return DenialTrackingState{}
}

// RecordDenial increments both consecutive and total denial counters.
func (s DenialTrackingState) RecordDenial() DenialTrackingState {
	return DenialTrackingState{
		ConsecutiveDenials: s.ConsecutiveDenials + 1,
		TotalDenials:       s.TotalDenials + 1,
	}
}

// RecordSuccess resets consecutive denial counter (total unchanged).
func (s DenialTrackingState) RecordSuccess() DenialTrackingState {
	if s.ConsecutiveDenials == 0 {
		return s
	}
	return DenialTrackingState{
		ConsecutiveDenials: 0,
		TotalDenials:       s.TotalDenials,
	}
}

// ShouldFallbackToPrompting reports whether the classifier should fall back
// to interactive user prompting due to too many denials.
func (s DenialTrackingState) ShouldFallbackToPrompting() bool {
	return s.ConsecutiveDenials >= DenialLimits.MaxConsecutive ||
		s.TotalDenials >= DenialLimits.MaxTotal
}
