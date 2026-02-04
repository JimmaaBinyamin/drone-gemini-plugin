package plugin

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GeminiClient handles direct API calls to Vertex AI
type GeminiClient struct {
	config *Config
}

// NewGeminiClient creates a new Gemini API client
func NewGeminiClient(cfg *Config) *GeminiClient {
	return &GeminiClient{
		config: cfg,
	}
}

// GenerateContentRequest represents the API request structure
type GenerateContentRequest struct {
	Contents []Content `json:"contents"`
}

// Content represents message content
type Content struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

// Part represents a content part (text or file)
type Part struct {
	Text     string    `json:"text,omitempty"`
	FileData *FileData `json:"fileData,omitempty"`
}

// FileData represents inline file data
type FileData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// GenerateContentResponse represents the API response
type GenerateContentResponse struct {
	Candidates    []Candidate    `json:"candidates"`
	UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
	Error         *APIError      `json:"error,omitempty"`
}

// UsageMetadata contains token usage information from API
type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
	ThoughtsTokenCount   int `json:"thoughtsTokenCount"` // Thinking tokens for reasoning models
}

// Candidate represents a response candidate
type Candidate struct {
	Content Content `json:"content"`
}

// APIError represents an API error
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// GenerateContent sends a prompt to Gemini and returns the response with usage stats
func (c *GeminiClient) GenerateContent() (string, *UsageStats, error) {
	cfg := c.config
	calc := NewCostCalculator(cfg.Model)

	if cfg.Debug {
		fmt.Printf("[DEBUG] Building context from directory: %s\n", cfg.Target)
	}

	// Build the full prompt with context
	fullPrompt, err := c.buildFullPrompt()
	if err != nil {
		return "", nil, err
	}

	// Estimate tokens locally before sending
	estimatedTokens := calc.EstimateTokens(fullPrompt)
	if cfg.Debug {
		fmt.Printf("[DEBUG] Estimated input tokens: %d\n", estimatedTokens)
	}

	// Build request
	reqBody := GenerateContentRequest{
		Contents: []Content{
			{
				Role: "user",
				Parts: []Part{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL based on auth mode
	authMode := cfg.DetectAuthMode()
	var apiURL string
	var authHeader string

	switch authMode {
	case AuthModeAPIKey:
		// Google AI Studio: Use generativelanguage.googleapis.com
		apiURL = fmt.Sprintf(
			"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
			cfg.Model,
			cfg.APIKey,
		)
		if cfg.Debug {
			fmt.Println("[DEBUG] Using Google AI Studio endpoint")
		}

	case AuthModeVertexAI:
		// Get OAuth token from service account
		token, err := c.getAccessToken()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get access token: %w", err)
		}
		authHeader = "Bearer " + token

		// Vertex AI: Different endpoint for global vs regional
		if cfg.GCPLocation == "global" {
			// Global: Use generativelanguage.googleapis.com with OAuth
			apiURL = fmt.Sprintf(
				"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
				cfg.Model,
			)
			if cfg.Debug {
				fmt.Printf("[DEBUG] Using Vertex AI global endpoint (Project: %s)\n", cfg.GCPProject)
			}
		} else {
			// Regional: Use {region}-aiplatform.googleapis.com
			apiURL = fmt.Sprintf(
				"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
				cfg.GCPLocation,
				cfg.GCPProject,
				cfg.GCPLocation,
				cfg.Model,
			)
			if cfg.Debug {
				fmt.Printf("[DEBUG] Using Vertex AI regional endpoint (Project: %s, Location: %s)\n", cfg.GCPProject, cfg.GCPLocation)
			}
		}

	default:
		return "", nil, ErrNoCredentials
	}

	if cfg.Debug {
		maskedURL := apiURL
		if cfg.APIKey != "" {
			maskedURL = strings.Replace(apiURL, cfg.APIKey, "***", 1)
		}
		fmt.Printf("[DEBUG] API URL: %s\n", maskedURL)
		fmt.Printf("[DEBUG] Request body length: %d bytes\n", len(jsonBody))
		fmt.Printf("[DEBUG] Timeout: %d seconds\n", cfg.Timeout)
	}

	// Make HTTP request with configurable timeout
	timeout := time.Duration(cfg.Timeout) * time.Second
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	if cfg.Debug {
		fmt.Printf("[DEBUG] Response status: %d\n", resp.StatusCode)
		fmt.Printf("[DEBUG] Response body: %s\n", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp GenerateContentResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return "", nil, fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	// Extract text from response
	var result strings.Builder
	for _, candidate := range apiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				result.WriteString(part.Text)
			}
		}
	}

	// Calculate usage statistics
	var usageStats *UsageStats
	if apiResp.UsageMetadata != nil {
		usageStats = calc.CalculateCost(
			apiResp.UsageMetadata.PromptTokenCount,
			apiResp.UsageMetadata.CandidatesTokenCount,
			apiResp.UsageMetadata.ThoughtsTokenCount,
		)
		usageStats.EstimatedInput = estimatedTokens
	} else {
		// Fallback: use estimates if API doesn't return usage metadata
		usageStats = calc.CalculateCost(estimatedTokens, calc.EstimateTokens(result.String()), 0)
		usageStats.EstimatedInput = estimatedTokens
	}

	return result.String(), usageStats, nil
}

// buildFullPrompt combines user prompt with git info and code context
func (c *GeminiClient) buildFullPrompt() (string, error) {
	cfg := c.config
	var promptBuilder strings.Builder

	// Add user prompt
	promptBuilder.WriteString(cfg.Prompt)
	promptBuilder.WriteString("\n\n")

	// Add git context if enabled
	if cfg.GitDiff {
		gitContext, err := c.buildGitContext()
		if err != nil {
			if cfg.Debug {
				fmt.Printf("[DEBUG] Failed to build git context: %v\n", err)
			}
			// Continue without git context
		} else if gitContext != "" {
			promptBuilder.WriteString(gitContext)
			promptBuilder.WriteString("\n")
		}
	}

	// Add code context
	codeContext, err := c.buildContext(cfg.Target)
	if err != nil {
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	if cfg.Debug {
		fmt.Printf("[DEBUG] Code context length: %d bytes\n", len(codeContext))
	}

	if codeContext != "" {
		promptBuilder.WriteString("=== Code Files ===\n")
		promptBuilder.WriteString(codeContext)
	}

	return promptBuilder.String(), nil
}

// buildGitContext builds context from git information
func (c *GeminiClient) buildGitContext() (string, error) {
	cfg := c.config
	git := NewGitAnalyzer(cfg.Target, cfg.Debug)

	if !git.IsGitRepository() {
		if cfg.Debug {
			fmt.Println("[DEBUG] Not a git repository, skipping git context")
		}
		return "", nil
	}

	// Detect commit SHA
	sha := git.DetectCommitSHA(cfg.GitCommitSHA)
	if sha == "" {
		return "", fmt.Errorf("could not detect commit SHA")
	}

	if cfg.Debug {
		fmt.Printf("[DEBUG] Analyzing commit: %s\n", sha)
	}

	// Build git context
	return git.BuildGitContext(sha)
}

// buildContext reads files from the target directory and builds context
func (c *GeminiClient) buildContext(targetDir string) (string, error) {
	cfg := c.config
	var context strings.Builder
	var fileCount int
	var totalSize int

	// Directories to exclude (common non-project directories)
	excludeDirs := map[string]bool{
		"context":      true, // sample/reference code
		"vendor":       true, // Go vendor
		"node_modules": true, // Node.js
		"dist":         true, // Build output
		"build":        true, // Build output
		"target":       true, // Maven/Gradle
		"__pycache__":  true, // Python cache
		".git":         true,
		".idea":        true,
		".vscode":      true,
	}

	// Supported file extensions
	extensions := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".md": true, ".yaml": true, ".yml": true, ".json": true,
		".sh": true, ".bash": true, ".dockerfile": true,
		".html": true, ".css": true, ".sql": true,
		".jsx": true, ".tsx": true, ".vue": true,
		".rb": true, ".php": true, ".rs": true,
	}

	// Get changed files for prioritization (if git diff enabled)
	var changedFiles map[string]bool
	if cfg.GitDiff {
		git := NewGitAnalyzer(targetDir, cfg.Debug)
		if git.IsGitRepository() {
			sha := git.DetectCommitSHA(cfg.GitCommitSHA)
			if files, err := git.GetChangedFiles(sha); err == nil {
				changedFiles = make(map[string]bool)
				for _, f := range files {
					changedFiles[f] = true
				}
				if cfg.Debug {
					fmt.Printf("[DEBUG] Found %d changed files to prioritize\n", len(changedFiles))
				}
			}
		}
	}

	// Collect files
	var priorityFiles []string
	var otherFiles []string

	err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if cfg.Debug {
				fmt.Printf("[DEBUG] Error accessing path %s: %v\n", path, err)
			}
			return nil
		}

		// Skip excluded directories
		if info.IsDir() {
			dirName := info.Name()

			// Don't skip the root target directory
			if path == targetDir {
				if cfg.Debug {
					fmt.Printf("[DEBUG] Processing root directory: %s\n", path)
				}
				return nil
			}

			// Skip hidden directories
			if strings.HasPrefix(dirName, ".") {
				if cfg.Debug {
					fmt.Printf("[DEBUG] Skipping hidden directory: %s\n", dirName)
				}
				return filepath.SkipDir
			}

			// Skip excluded directories
			if excludeDirs[dirName] {
				if cfg.Debug {
					fmt.Printf("[DEBUG] Skipping excluded directory: %s\n", dirName)
				}
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(path))
		if !extensions[ext] && info.Name() != "Dockerfile" {
			return nil
		}

		// Skip large files (> 100KB)
		if info.Size() > 100*1024 {
			if cfg.Debug {
				fmt.Printf("[DEBUG] Skipping large file: %s (%d bytes)\n", path, info.Size())
			}
			return nil
		}

		relPath, _ := filepath.Rel(targetDir, path)

		// Prioritize changed files
		if changedFiles != nil && changedFiles[relPath] {
			priorityFiles = append(priorityFiles, path)
		} else {
			otherFiles = append(otherFiles, path)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// Process files: priority files first, then other files
	allFiles := append(priorityFiles, otherFiles...)

	for _, path := range allFiles {
		// Check file count limit
		if cfg.MaxFiles > 0 && fileCount >= cfg.MaxFiles {
			if cfg.Debug {
				fmt.Printf("[DEBUG] Reached max file limit (%d), stopping\n", cfg.MaxFiles)
			}
			context.WriteString(fmt.Sprintf("\n... [Truncated: reached max file limit of %d files] ...\n", cfg.MaxFiles))
			break
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Check context size limit
		if cfg.MaxContextSize > 0 && totalSize+len(content) > cfg.MaxContextSize {
			if cfg.Debug {
				fmt.Printf("[DEBUG] Reached max context size (%d bytes), stopping\n", cfg.MaxContextSize)
			}
			context.WriteString(fmt.Sprintf("\n... [Truncated: reached max context size of %d bytes] ...\n", cfg.MaxContextSize))
			break
		}

		// Add to context
		relPath, _ := filepath.Rel(targetDir, path)
		if cfg.Debug {
			fmt.Printf("[DEBUG] Including file: %s (%d bytes)\n", relPath, len(content))
		}
		context.WriteString(fmt.Sprintf("\n--- File: %s ---\n", relPath))
		context.WriteString(string(content))
		context.WriteString("\n")
		fileCount++
		totalSize += len(content)
	}

	if cfg.Debug {
		fmt.Printf("[DEBUG] Total files included: %d, Total size: %d bytes\n", fileCount, totalSize)
	}

	return context.String(), nil
}

// ServiceAccountCredentials represents GCP service account JSON structure
type ServiceAccountCredentials struct {
	Type                    string `json:"type"`
	ProjectID               string `json:"project_id"`
	PrivateKeyID            string `json:"private_key_id"`
	PrivateKey              string `json:"private_key"`
	ClientEmail             string `json:"client_email"`
	ClientID                string `json:"client_id"`
	AuthURI                 string `json:"auth_uri"`
	TokenURI                string `json:"token_uri"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url"`
	ClientX509CertURL       string `json:"client_x509_cert_url"`
}

// TokenResponse represents OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// getAccessToken gets an OAuth access token from service account credentials
func (c *GeminiClient) getAccessToken() (string, error) {
	cfg := c.config

	// Parse service account credentials
	var creds ServiceAccountCredentials
	if err := json.Unmarshal([]byte(cfg.GCPCredentials), &creds); err != nil {
		return "", fmt.Errorf("failed to parse service account credentials: %w", err)
	}

	// Create JWT for token exchange
	// Include both scopes to support regional (aiplatform) and global (generativelanguage) endpoints
	now := time.Now()
	claims := map[string]interface{}{
		"iss":   creds.ClientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/generative-language",
		"aud":   creds.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}

	// Build JWT token
	token, err := c.signJWT(claims, creds.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to create JWT: %w", err)
	}

	// Exchange JWT for access token
	resp, err := http.PostForm(creds.TokenURI, map[string][]string{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {token},
	})
	if err != nil {
		return "", fmt.Errorf("failed to exchange JWT for access token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// signJWT creates a signed JWT token using RS256
func (c *GeminiClient) signJWT(claims map[string]interface{}, privateKeyPEM string) (string, error) {
	// Parse private key
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	// Create JWT header
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64

	// Sign with RSA-SHA256
	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureB64, nil
}
