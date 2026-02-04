package plugin

import "errors"

var (
	// ErrPromptRequired is returned when no prompt is provided
	ErrPromptRequired = errors.New("prompt is required: set PLUGIN_PROMPT")

	// ErrNoCredentials is returned when no authentication credentials are provided
	ErrNoCredentials = errors.New("no credentials provided: set PLUGIN_API_KEY for Google AI Studio or PLUGIN_GCP_CREDENTIALS + PLUGIN_GCP_PROJECT for Vertex AI")

	// ErrProjectRequired is returned when using Vertex AI without a project ID
	ErrProjectRequired = errors.New("GCP project ID is required for Vertex AI: set PLUGIN_GCP_PROJECT")
)
