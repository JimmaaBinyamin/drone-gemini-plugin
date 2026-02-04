package plugin

import (
	"fmt"
	"strings"
)

// Plugin represents the drone-gemini-plugin
type Plugin struct {
	config Config
}

// New creates a new plugin instance
func New(cfg Config) *Plugin {
	return &Plugin{
		config: cfg,
	}
}

// Exec runs the plugin and returns any error encountered
func (p *Plugin) Exec() error {
	// Validate configuration
	if err := p.config.Validate(); err != nil {
		return err
	}

	// Detect authentication mode
	authMode := p.config.DetectAuthMode()

	// Display configuration summary
	p.displayConfig(authMode)

	// Execute AI analysis
	fmt.Println("Executing AI analysis...")
	fmt.Println()

	// Create Gemini client and generate content
	client := NewGeminiClient(&p.config)
	output, usageStats, err := client.GenerateContent()
	if err != nil {
		return err
	}

	// Display AI output
	fmt.Println("=== AI Analysis Result ===")
	fmt.Println()
	fmt.Println(output)

	// Display cost statistics
	if usageStats != nil {
		fmt.Print(usageStats.FormatCostSummary())
	}

	return nil
}

// displayConfig shows the current configuration
func (p *Plugin) displayConfig(authMode AuthMode) {
	fmt.Println()
	fmt.Println("--- Configuration ---")
	fmt.Printf("Target: %s\n", p.config.Target)
	fmt.Printf("Model: %s\n", p.config.Model)
	fmt.Printf("Prompt: %s\n", truncateString(p.config.Prompt, 100))
	fmt.Printf("Timeout: %ds\n", p.config.Timeout)

	if p.config.GitDiff {
		fmt.Println("Git Diff: enabled")
	}

	if p.config.MaxFiles > 0 {
		fmt.Printf("Max Files: %d\n", p.config.MaxFiles)
	}

	if authMode == AuthModeVertexAI || p.config.GCPProject != "" {
		fmt.Printf("GCP Project: %s\n", p.config.GCPProject)
		fmt.Printf("GCP Location: %s\n", p.config.GCPLocation)
	}

	fmt.Println()
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// maskAPIKey masks an API key for safe logging
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
