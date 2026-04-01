package mode

// DefaultAutoModeRules contains the built-in rules for the Auto Mode
// LLM classifier. These are prepended before any user-supplied rules.
var DefaultAutoModeRules = []AutoModeRule{
	{Type: "allow", Description: "Reading files or directories"},
	{Type: "allow", Description: "Running tests"},
	{Type: "allow", Description: "Searching code with grep or glob"},
	{Type: "allow", Description: "Fetching public web pages"},
	{Type: "allow", Description: "Listing directory contents"},
	{Type: "allow", Description: "Viewing git status or log"},
	{Type: "soft_deny", Description: "Deleting files"},
	{Type: "soft_deny", Description: "Pushing to remote git repositories"},
	{Type: "soft_deny", Description: "Installing packages system-wide"},
	{Type: "soft_deny", Description: "Modifying system configuration files"},
	{Type: "soft_deny", Description: "Running database migrations"},
	{Type: "environment", Description: "The agent operates on the user's local machine"},
	{Type: "environment", Description: "Changes affect the real filesystem"},
}
