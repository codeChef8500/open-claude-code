package mode

// ClassifierVerdict is the result of the Auto Mode LLM classifier.
type ClassifierVerdict string

const (
	VerdictAllow    ClassifierVerdict = "allow"
	VerdictSoftDeny ClassifierVerdict = "soft_deny"
	VerdictDeny     ClassifierVerdict = "deny"
)

// AutoModeRule is a single rule fed to the classifier system prompt.
type AutoModeRule struct {
	Type        string // "allow" | "soft_deny" | "environment"
	Description string
}

// ClassifierResult bundles a verdict with an explanatory reason.
type ClassifierResult struct {
	Verdict ClassifierVerdict
	Reason  string
}
