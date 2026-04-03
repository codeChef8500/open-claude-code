package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wall-ai/agent-engine/internal/util"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check system health and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func runDoctor() error {
	var sb strings.Builder
	sb.WriteString("Agent Engine Doctor\n")
	sb.WriteString("===================\n\n")

	// Version
	sb.WriteString(fmt.Sprintf("Version:    %s\n", Version))
	sb.WriteString(fmt.Sprintf("Go:         %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString("\n")

	// Working directory
	wd, _ := os.Getwd()
	sb.WriteString(fmt.Sprintf("Work dir:   %s\n", wd))

	// Config
	if err := util.InitConfig(); err != nil {
		sb.WriteString(fmt.Sprintf("Config:     ✗ %v\n", err))
	} else {
		sb.WriteString("Config:     ✓ loaded\n")
	}

	// AppConfig
	appCfg, err := util.LoadAppConfig(wd)
	if err != nil {
		sb.WriteString(fmt.Sprintf("AppConfig:  ✗ %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("Provider:   %s\n", appCfg.Provider))
		sb.WriteString(fmt.Sprintf("Model:      %s\n", appCfg.Model))
		sb.WriteString(fmt.Sprintf("Base URL:   %s\n", maskURL(appCfg.BaseURL)))

		// API key check
		if appCfg.APIKey != "" {
			sb.WriteString("API Key:    ✓ set\n")
		} else {
			sb.WriteString("API Key:    ✗ not set\n")
		}

		sb.WriteString(fmt.Sprintf("Permission: %s\n", appCfg.PermissionMode))

		// Config paths
		if len(appCfg.ConfigPaths) > 0 {
			sb.WriteString("\nLoaded config files:\n")
			for _, p := range appCfg.ConfigPaths {
				sb.WriteString(fmt.Sprintf("  • %s\n", p))
			}
		}

		// MCP servers
		if len(appCfg.MCPServers) > 0 {
			sb.WriteString(fmt.Sprintf("\nMCP servers: %d configured\n", len(appCfg.MCPServers)))
			for name, cfg := range appCfg.MCPServers {
				status := "enabled"
				if cfg.Disabled {
					status = "disabled"
				}
				sb.WriteString(fmt.Sprintf("  • %s (%s)\n", name, status))
			}
		}
	}

	fmt.Print(sb.String())
	return nil
}

func maskURL(u string) string {
	if u == "" {
		return "(default)"
	}
	// Show host but mask path details
	if len(u) > 40 {
		return u[:40] + "..."
	}
	return u
}
