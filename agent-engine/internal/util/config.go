package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// InitConfig loads configuration from (in order of precedence):
//  1. Environment variables (prefixed with AGENT_ENGINE_)
//  2. ~/.claude/config.json
//  3. Built-in defaults
func InitConfig() error {
	viper.SetEnvPrefix("AGENT_ENGINE")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("provider", "anthropic")
	viper.SetDefault("model", "claude-sonnet-4-5")
	viper.SetDefault("max_tokens", 8192)
	viper.SetDefault("thinking_budget", 0)
	viper.SetDefault("http_port", 8080)
	viper.SetDefault("verbose", false)
	viper.SetDefault("auto_mode", false)
	viper.SetDefault("max_cost_usd", 0.0)
	viper.SetDefault("slow_threshold_ms", 100)

	// Config file
	home, err := os.UserHomeDir()
	if err != nil {
		return nil // proceed with env + defaults
	}
	cfgPath := filepath.Join(home, ".claude", "config.json")
	viper.SetConfigFile(cfgPath)
	viper.SetConfigType("json")

	if err := viper.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read config: %w", err)
		}
		// No config file — use defaults
	}

	return nil
}

// GetString is a typed accessor for string config values.
func GetString(key string) string { return viper.GetString(key) }

// GetInt is a typed accessor for int config values.
func GetInt(key string) int { return viper.GetInt(key) }

// GetBool is a typed accessor for bool config values.
func GetBoolConfig(key string) bool { return viper.GetBool(key) }

// GetFloat64 is a typed accessor for float64 config values.
func GetFloat64(key string) float64 { return viper.GetFloat64(key) }
