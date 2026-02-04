package plugin

// Config holds the plugin configuration from environment variables.
// Drone CI injects these as PLUGIN_* environment variables.
type Config struct {
	// Prompt is the instruction for the AI (required)
	Prompt string `envconfig:"PROMPT" required:"true"`

	// Target is the file or directory to scan (optional, defaults to ".")
	Target string `envconfig:"TARGET" default:"."`

	// Model specifies which AI model to use (default: gemini-2.5-pro for 1M context)
	Model string `envconfig:"MODEL" default:"gemini-2.5-pro"`

	// APIKey for Google AI Studio authentication (Scenario A)
	APIKey string `envconfig:"API_KEY"`

	// GCPCredentials is the raw JSON credentials string for Vertex AI (Scenario B)
	GCPCredentials string `envconfig:"GCP_CREDENTIALS"`

	// GCPProject is the Google Cloud project ID for Vertex AI
	GCPProject string `envconfig:"GCP_PROJECT"`

	// GCPLocation is the Google Cloud location for Vertex AI (e.g., us-central1)
	GCPLocation string `envconfig:"GCP_LOCATION" default:"us-central1"`

	// Debug enables debug output
	Debug bool `envconfig:"DEBUG" default:"false"`

	// Timeout in seconds for API calls (default 300s = 5 minutes)
	Timeout int `envconfig:"TIMEOUT" default:"300"`

	// GitDiff enables analyzing the last commit diff
	GitDiff bool `envconfig:"GIT_DIFF" default:"false"`

	// GitCommitSHA to analyze (auto-detected from DRONE_COMMIT_SHA if empty)
	GitCommitSHA string `envconfig:"GIT_COMMIT_SHA"`

	// MaxFiles limits the number of files to include (0 = no limit)
	MaxFiles int `envconfig:"MAX_FILES" default:"50"`

	// MaxContextSize limits total context size in bytes (default 500KB)
	MaxContextSize int `envconfig:"MAX_CONTEXT_SIZE" default:"512000"`
}

// AuthMode represents the authentication mode detected from configuration
type AuthMode int

const (
	AuthModeNone AuthMode = iota
	AuthModeAPIKey
	AuthModeVertexAI
)

// DetectAuthMode automatically detects which authentication mode to use
// - APIKey alone = Google AI Studio (simplest)
// - GCPCredentials + GCPProject = Vertex AI with Service Account (enterprise)
func (c *Config) DetectAuthMode() AuthMode {
	// Scenario A: API Key (Google AI Studio) - simplest option
	if c.APIKey != "" {
		return AuthModeAPIKey
	}

	// Scenario B: Vertex AI with Service Account credentials
	if c.GCPCredentials != "" && c.GCPProject != "" {
		return AuthModeVertexAI
	}

	return AuthModeNone
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Prompt == "" {
		return ErrPromptRequired
	}

	authMode := c.DetectAuthMode()
	if authMode == AuthModeNone {
		return ErrNoCredentials
	}

	if authMode == AuthModeVertexAI && c.GCPProject == "" {
		return ErrProjectRequired
	}

	return nil
}
