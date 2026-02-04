package plugin

import (
	"testing"
)

func TestConfig_DetectAuthMode(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected AuthMode
	}{
		{
			name: "API Key mode",
			config: Config{
				APIKey: "test-api-key",
			},
			expected: AuthModeAPIKey,
		},
		{
			name: "Vertex AI mode",
			config: Config{
				GCPCredentials: `{"type":"service_account"}`,
				GCPProject:     "my-project",
			},
			expected: AuthModeVertexAI,
		},
		{
			name: "API Key takes precedence when both provided",
			config: Config{
				APIKey:         "test-api-key",
				GCPCredentials: `{"type":"service_account"}`,
				GCPProject:     "my-project",
			},
			expected: AuthModeAPIKey,
		},
		{
			name: "No credentials",
			config: Config{
				Prompt: "test",
			},
			expected: AuthModeNone,
		},
		{
			name: "GCP credentials without project",
			config: Config{
				GCPCredentials: `{"type":"service_account"}`,
			},
			expected: AuthModeNone, // No valid auth mode without project
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.DetectAuthMode()
			if got != tt.expected {
				t.Errorf("DetectAuthMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorType   error
	}{
		{
			name: "Valid config with API key",
			config: Config{
				Prompt: "Review this code",
				APIKey: "test-key",
			},
			expectError: false,
		},
		{
			name: "Valid config with Vertex AI",
			config: Config{
				Prompt:         "Review this code",
				GCPCredentials: `{"type":"service_account"}`,
				GCPProject:     "my-project",
			},
			expectError: false,
		},
		{
			name: "Missing prompt",
			config: Config{
				APIKey: "test-key",
			},
			expectError: true,
			errorType:   ErrPromptRequired,
		},
		{
			name: "Missing credentials",
			config: Config{
				Prompt: "Review this code",
			},
			expectError: true,
			errorType:   ErrNoCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if tt.errorType != nil && err != tt.errorType {
					t.Errorf("Validate() error = %v, want %v", err, tt.errorType)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exact", 5, "exact"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncateString(tt.input, tt.maxLen)
		if got != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"short", "***"},
		{"12345678", "***"},
		{"1234567890", "1234**7890"},
		{"abcdefghijklmnop", "abcd********mnop"},
	}

	for _, tt := range tests {
		got := maskAPIKey(tt.input)
		if got != tt.expected {
			t.Errorf("maskAPIKey(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestCostCalculator(t *testing.T) {
	calc := NewCostCalculator("gemini-2.5-pro")

	// Test cost calculation
	stats := calc.CalculateCost(1000, 100, 500)

	if stats.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", stats.InputTokens)
	}
	if stats.OutputTokens != 100 {
		t.Errorf("OutputTokens = %d, want 100", stats.OutputTokens)
	}
	if stats.ThoughtsTokens != 500 {
		t.Errorf("ThoughtsTokens = %d, want 500", stats.ThoughtsTokens)
	}
	if stats.TotalTokens != 1600 {
		t.Errorf("TotalTokens = %d, want 1600", stats.TotalTokens)
	}
	if stats.TotalCost <= 0 {
		t.Error("TotalCost should be greater than 0")
	}
}

func TestEstimateTokens(t *testing.T) {
	calc := NewCostCalculator("gemini-2.5-pro")

	// ~3 chars per token
	text := "Hello world, this is a test string"
	estimate := calc.EstimateTokens(text)

	if estimate <= 0 {
		t.Error("EstimateTokens should return positive value")
	}
	// 34 chars / 3 ≈ 11 tokens
	if estimate < 5 || estimate > 20 {
		t.Errorf("EstimateTokens(%q) = %d, expected around 11", text, estimate)
	}
}
