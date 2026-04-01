package util

import (
	"context"
	"fmt"
	"runtime"
	"strings"
)

// GetSecureValue retrieves a secret value for key from the OS secure storage:
//   - macOS  : Keychain (via `security` CLI)
//   - Windows: Credential Manager (via PowerShell DPAPI)
//   - Linux  : attempts secret-tool (GNOME Keyring / KWallet), then falls back
//              to an encrypted file at ~/.claude/secrets/<key>.enc
func GetSecureValue(ctx context.Context, key string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return readFromKeychain(ctx, key)
	case "windows":
		return readFromWindowsCredentialManager(ctx, key)
	default:
		// Linux / BSDs
		val, err := readFromSecretTool(ctx, key)
		if err == nil {
			return val, nil
		}
		return readFromEncryptedFile(key)
	}
}

// SetSecureValue stores value under key in the OS secure storage.
func SetSecureValue(ctx context.Context, key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return writeToKeychain(ctx, key, value)
	case "windows":
		return writeToWindowsCredentialManager(ctx, key, value)
	default:
		return writeToEncryptedFile(key, value)
	}
}

// ─── macOS Keychain ───────────────────────────────────────────────────────────

func readFromKeychain(ctx context.Context, key string) (string, error) {
	result, err := Exec(ctx, fmt.Sprintf(
		"security find-generic-password -s %s -w", ShellQuote("agent-engine:"+key),
	), nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func writeToKeychain(ctx context.Context, key, value string) error {
	_, err := Exec(ctx, fmt.Sprintf(
		"security add-generic-password -U -s %s -w %s",
		ShellQuote("agent-engine:"+key),
		ShellQuote(value),
	), nil)
	return err
}

// ─── Windows Credential Manager ──────────────────────────────────────────────

func readFromWindowsCredentialManager(ctx context.Context, key string) (string, error) {
	ps := fmt.Sprintf(
		`$c = [System.Net.CredentialCache]::GetCredential("agent-engine:%s","","Generic"); if($c){$c.Password}`,
		key,
	)
	result, err := Exec(ctx, "powershell -NoProfile -Command "+ShellQuote(ps), nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func writeToWindowsCredentialManager(ctx context.Context, key, value string) error {
	ps := fmt.Sprintf(
		`$c = New-Object System.Net.NetworkCredential("agent-engine","%s",""); `+
			`[System.Net.CredentialCache]::Add([System.Uri]"agent-engine:%s","Generic",$c)`,
		value, key,
	)
	_, err := Exec(ctx, "powershell -NoProfile -Command "+ShellQuote(ps), nil)
	return err
}

// ─── Linux: secret-tool ───────────────────────────────────────────────────────

func readFromSecretTool(ctx context.Context, key string) (string, error) {
	result, err := Exec(ctx, fmt.Sprintf(
		"secret-tool lookup application agent-engine key %s", ShellQuote(key),
	), nil)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(result.Stdout)
	if v == "" {
		return "", fmt.Errorf("secret not found")
	}
	return v, nil
}

// ─── Encrypted file fallback ──────────────────────────────────────────────────

func readFromEncryptedFile(key string) (string, error) {
	path := secureFilePath(key)
	content, err := ReadTextFile(path)
	if err != nil {
		return "", fmt.Errorf("secure storage: %w", err)
	}
	return strings.TrimSpace(content), nil
}

func writeToEncryptedFile(key, value string) error {
	path := secureFilePath(key)
	return WriteTextContent(path, value)
}

func secureFilePath(key string) string {
	home, _ := expandHomePath()
	return JoinPath(home, ".claude", "secrets", sanitiseKey(key)+".enc")
}

func sanitiseKey(key string) string {
	var sb strings.Builder
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}

func expandHomePath() (string, error) {
	return ExpandPath("~"), nil
}
