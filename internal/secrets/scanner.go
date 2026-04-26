// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package secrets

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/openartifact/openartifact/internal/database/models"
	"github.com/openartifact/openartifact/pkg/logger"
)

// SecretPattern represents a pattern for detecting secrets.
type SecretPattern struct {
	Name        string
	Description string
	Severity    string
	Regex       *regexp.Regexp
}

// Scanner scans text for secrets.
type Scanner struct {
	log      *logger.Logger
	patterns []SecretPattern
}

// NewScanner creates a new secret scanner.
func NewScanner(log *logger.Logger) *Scanner {
	return &Scanner{
		log:      log,
		patterns: defaultPatterns,
	}
}

//nolint:gochecknoglobals // immutable default secret detection patterns
var defaultPatterns = []SecretPattern{
	{
		Name:        "aws_access_key",
		Description: "AWS Access Key ID",
		Severity:    "critical",
		Regex:       regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`),
	},
	{
		Name:        "aws_secret_key",
		Description: "AWS Secret Access Key",
		Severity:    "critical",
		Regex:       regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}['\"][0-9a-zA-Z/+]{40}['\"]`),
	},
	{
		Name:        "github_token",
		Description: "GitHub Personal Access Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
	},
	{
		Name:        "github_oauth",
		Description: "GitHub OAuth Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`gho_[0-9a-zA-Z]{36}`),
	},
	{
		Name:        "github_app_token",
		Description: "GitHub App Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`(ghu|ghs)_[0-9a-zA-Z]{36}`),
	},
	{
		Name:        "slack_token",
		Description: "Slack API Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})?`),
	},
	{
		Name:        "stripe_key",
		Description: "Stripe API Key",
		Severity:    "critical",
		Regex:       regexp.MustCompile(`(?:r|s)k_live_[0-9a-zA-Z]{24}`),
	},
	{
		Name:        "private_key",
		Description: "Private Key",
		Severity:    "critical",
		Regex:       regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`),
	},
	{
		Name:        "generic_api_key",
		Description: "Generic API Key",
		Severity:    "medium",
		Regex:       regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|api[_-]?secret)['\"\s]*[:=]['\"\s]*([a-zA-Z0-9_\-]{20,})`),
	},
	{
		Name:        "generic_password",
		Description: "Generic Password",
		Severity:    "medium",
		Regex:       regexp.MustCompile(`(?i)(?:password|passwd|pwd)['\"\s]*[:=]['\"\s]*([^\s'\"]{8,})`),
	},
	{
		Name:        "jwt_token",
		Description: "JWT Token",
		Severity:    "medium",
		Regex:       regexp.MustCompile(`eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}`),
	},
	{
		Name:        "google_api_key",
		Description: "Google API Key",
		Severity:    "high",
		Regex:       regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	},
	{
		Name:        "npm_token",
		Description: "npm Access Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`npm_[a-zA-Z0-9]{36}`),
	},
	{
		Name:        "docker_hub_token",
		Description: "Docker Hub Token",
		Severity:    "high",
		Regex:       regexp.MustCompile(`dckr_pat_[a-zA-Z0-9_-]{27}`),
	},
}

// ScanText scans a text string for secrets.
func (s *Scanner) ScanText(text string) []models.SecretFinding {
	var findings []models.SecretFinding

	for _, pattern := range s.patterns {
		matches := pattern.Regex.FindAllStringIndex(text, -1)
		for _, match := range matches {
			start, end := match[0], match[1]
			matchedText := text[start:end]

			finding := models.SecretFinding{
				Type:          pattern.Name,
				Description:   pattern.Description,
				Severity:      pattern.Severity,
				RedactedValue: redactValue(matchedText),
				LineHint:      getLineHint(text, start, end),
			}
			findings = append(findings, finding)
		}
	}

	return findings
}

// ScanMetadata scans artifact metadata map for secrets.
func (s *Scanner) ScanMetadata(metadata map[string]any) []models.SecretFinding {
	// Convert metadata to JSON string and scan
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		s.log.Warnw("failed to marshal metadata for secret scanning", "error", err)
		return nil
	}

	return s.ScanText(string(metadataJSON))
}

// redactValue returns first 4 chars + "****".
func redactValue(value string) string {
	if len(value) <= 4 {
		return "****"
	}

	return value[:4] + "****"
}

// getLineHint returns up to 100 chars around the match, with the secret replaced by "****".
func getLineHint(text string, start, end int) string {
	const contextSize = 50

	// Find line boundaries
	lineStart := start
	for lineStart > 0 && text[lineStart-1] != '\n' {
		lineStart--
	}

	lineEnd := end
	for lineEnd < len(text) && text[lineEnd] != '\n' {
		lineEnd++
	}

	// Get context around the match
	contextStart := max(start-contextSize, lineStart)

	contextEnd := min(end+contextSize, lineEnd)

	// Extract the hint
	hint := text[contextStart:contextEnd]

	// Replace the secret with "****"
	relativeStart := start - contextStart
	relativeEnd := end - contextStart
	hint = hint[:relativeStart] + "****" + hint[relativeEnd:]

	// Clean up whitespace
	hint = strings.TrimSpace(hint)

	// Limit to 100 chars
	if len(hint) > 100 {
		hint = hint[:100] + "..."
	}

	return hint
}
