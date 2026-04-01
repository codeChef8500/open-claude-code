package util

import (
	"os"
	"strings"
)

// IsEnvTruthy returns true if the environment variable value represents a
// truthy boolean: "1", "true", "yes", or "on" (case-insensitive).
func IsEnvTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// EnvBool looks up an environment variable and interprets it as a bool.
func EnvBool(key string) bool {
	return IsEnvTruthy(os.Getenv(key))
}

// EnvString returns the value of the environment variable, or defaultVal if unset/empty.
func EnvString(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// GetSafeEnvVars returns a filtered copy of os.Environ() with sensitive variables
// removed and safety overrides injected.
func GetSafeEnvVars() []string {
	sensitive := map[string]bool{
		"ANTHROPIC_API_KEY": true,
		"OPENAI_API_KEY":    true,
		"AWS_ACCESS_KEY_ID": true,
		"AWS_SECRET_ACCESS_KEY": true,
		"GOOGLE_APPLICATION_CREDENTIALS": true,
		"GITHUB_TOKEN": true,
		"NPM_TOKEN":    true,
	}

	result := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		key := kv[:idx]
		if sensitive[key] {
			continue
		}
		result = append(result, kv)
	}

	// Append safety overrides.
	result = append(result, SafeEnvVars...)
	return result
}

// MustGetenv returns the value of key or panics with a helpful message if unset.
func MustGetenv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable not set: " + key)
	}
	return v
}
