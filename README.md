# drone-gemini-plugin

English | [中文](README_zh.md)

A Drone CI/CD plugin that integrates Google Gemini AI for automated code analysis, review, and documentation.

## Features

- **AI Code Review**: Automated code analysis using Google Gemini
- **Git Diff Analysis**: Focus on changed files to reduce token costs
- **Cost Tracking**: Real-time token usage and cost estimation
- **Dual Auth Support**: Gemini API Key or Vertex AI Service Account
- **Global & Regional**: Supports both global and regional Vertex AI endpoints

## Quick Start

### Option A: Gemini API Key (Simplest)

Get a free API key from [Google AI Studio](https://aistudio.google.com/apikey).

```bash
# Add secret to Drone
drone secret add --repository your-org/your-repo \
  --name gemini_api_key --data "AIzaSy..."
```

```yaml
kind: pipeline
type: docker
name: ai-review

steps:
  - name: code-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: "Review this code for bugs and security issues"
      model: gemini-2.5-flash
      api_key:
        from_secret: gemini_api_key
```

### Option B: Vertex AI with Service Account

For enterprise use or when running on AWS/Azure/on-prem infrastructure.

```bash
# 1. Create GCP Service Account with Vertex AI User role
gcloud iam service-accounts create gemini-sa
gcloud projects add-iam-policy-binding YOUR_PROJECT \
  --member="serviceAccount:gemini-sa@YOUR_PROJECT.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"

# 2. Download key and add to Drone
gcloud iam service-accounts keys create sa-key.json \
  --iam-account=gemini-sa@YOUR_PROJECT.iam.gserviceaccount.com
drone secret add --repository your-org/your-repo \
  --name gcp_credentials --data @sa-key.json
```

```yaml
kind: pipeline
type: docker
name: ai-review

steps:
  - name: code-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: "Review this code for security issues"
      model: gemini-3-pro-preview
      gcp_project: your-gcp-project-id
      gcp_location: global
      gcp_credentials:
        from_secret: gcp_credentials
```

## Configuration

| Parameter | Environment Variable | Type | Default | Description |
|-----------|---------------------|------|---------|-------------|
| `prompt` | `PLUGIN_PROMPT` | string | **required** | AI instruction/prompt |
| `target` | `PLUGIN_TARGET` | string | `.` | Directory or file to analyze |
| `model` | `PLUGIN_MODEL` | string | `gemini-2.5-pro` | Model to use |
| `api_key` | `PLUGIN_API_KEY` | string | | Gemini API Key (Google AI Studio) |
| `gcp_project` | `PLUGIN_GCP_PROJECT` | string | | GCP Project ID (Vertex AI) |
| `gcp_location` | `PLUGIN_GCP_LOCATION` | string | `us-central1` | GCP Location (`global` for gemini-3-*) |
| `gcp_credentials` | `PLUGIN_GCP_CREDENTIALS` | string | | Service Account JSON content |
| `git_diff` | `PLUGIN_GIT_DIFF` | bool | `false` | Analyze only git changes |
| `max_files` | `PLUGIN_MAX_FILES` | int | `50` | Maximum files to include |
| `max_context_size` | `PLUGIN_MAX_CONTEXT_SIZE` | int | `500000` | Max context size in bytes |
| `timeout` | `PLUGIN_TIMEOUT` | int | `300` | Timeout in seconds |
| `debug` | `PLUGIN_DEBUG` | bool | `false` | Enable debug output |

## Examples

### PR Code Review

```yaml
steps:
  - name: ai-review
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: |
        Review this PR for:
        - Security vulnerabilities
        - Performance issues
        - Code quality and maintainability
      git_diff: true
      model: gemini-2.5-flash
      api_key:
        from_secret: gemini_api_key
    when:
      event: pull_request
```

### Security Audit with Vertex AI

```yaml
steps:
  - name: security-audit
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: |
        Perform comprehensive security audit:
        1. Check for SQL/NoSQL injection
        2. Review authentication and authorization
        3. Identify sensitive data exposure
      model: gemini-3-pro-preview
      gcp_project: my-project
      gcp_location: global
      gcp_credentials:
        from_secret: gcp_credentials
```

### Generate Release Notes

```yaml
steps:
  - name: release-notes
    image: ghcr.io/jimmaabinyamin/drone-gemini-plugin
    settings:
      prompt: "Generate release notes from recent commits in CHANGELOG format"
      git_diff: true
      api_key:
        from_secret: gemini_api_key
    when:
      event: tag
```

## Supported Models

| Model | Context | Best For | Pricing |
|-------|---------|----------|---------|
| `gemini-2.5-pro` | 1M tokens | Large codebases, complex analysis | $1.25-$2.50/1M input |
| `gemini-2.5-flash` | 1M tokens | Fast, cost-effective reviews | $0.15-$0.30/1M input |
| `gemini-3-pro-preview` | 1M tokens | Latest model (global region) | $4/1M input |

## Cost Tracking

The plugin displays token usage and estimated costs after each run:

```
+--------------------------------------------------------------+
|                    Token Usage Statistics                     |
+--------------------------------------------------------------+
|  Model: Gemini 2.5 Pro                                        |
|  Input Tokens: 4199  |  Output Tokens: 72                     |
|  Thinking Tokens: 1576                                        |
|  Total Cost: $0.021729                                        |
+--------------------------------------------------------------+
```

## Local Testing

```bash
# Build the plugin
go build -o drone-gemini-plugin .

# Test with Gemini API Key
PLUGIN_PROMPT="Describe this project" \
PLUGIN_API_KEY="your-api-key" \
PLUGIN_MODEL="gemini-2.5-flash" \
./drone-gemini-plugin

# Test with Vertex AI (global region)
PLUGIN_PROMPT="Describe this project" \
PLUGIN_GCP_PROJECT="your-project-id" \
PLUGIN_GCP_LOCATION="global" \
PLUGIN_GCP_CREDENTIALS="$(cat service-account.json)" \
PLUGIN_MODEL="gemini-3-pro-preview" \
./drone-gemini-plugin
```

## Building Docker Image

```bash
# Build the image
docker build -t ghcr.io/jimmaabinyamin/drone-gemini-plugin .

# Or with custom registry
docker build -t your-registry.com/drone-gemini:latest .
docker push your-registry.com/drone-gemini:latest
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
