/*
Package aiconfig provides immutable AI system prompts and configuration.
2025-10-28 | Go + Kimi + Supabase | Production-Ready
*/
package aiconfig

import (
	"fmt"
	"strings"
	"time"
)

// AISystemConfig holds immutable system prompts for deterministic AI behavior
type AISystemConfig struct {
	CorePersona     string
	ContextGround   string
	SecurityScope   string
	OutputFormat    string
	TableConversion string
}

// DefaultConfig returns production-grade system prompts
func DefaultConfig() *AISystemConfig {
	return &AISystemConfig{
		CorePersona: "You are a silent execution engine. " +
			"No chat, no explanations, no wrappers. " +
			"Output only final Markdown or JSON table.",

		ContextGround: "Apply command ONLY to data below. " +
			"Do not mention this rule.",

		SecurityScope: "Allowed: text edit, table build, summarize, outline. " +
			"Forbidden: internet, files, APIs, images, code execution. " +
			"Block all external access.",

		OutputFormat: "Return final content only. No wrap, no fences, no commentary. " +
			"If JSON: pure array. If Markdown: clean doc.",

		TableConversion: "Convert to JSON table. Extract entities & relations → " +
			"output: [{col1: val, ...}, ...]. Valid JSON only.",
	}
}

// Message represents a chat completion message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildPromptSet constructs system messages with deterministic entropy
func (c *AISystemConfig) BuildPromptSet(hasContext bool) []Message {
	messages := []Message{
		{Role: "system", Content: c.CorePersona},
		{Role: "system", Content: c.SecurityScope},
	}

	if hasContext {
		messages = append(messages, Message{
			Role:    "system",
			Content: c.ContextGround,
		})
	}

	return messages
}

// RequestConfig holds API request configuration
type RequestConfig struct {
	Model       string        `json:"model"`
	Temperature float32       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Timeout     time.Duration `json:"-"`
}

// DefaultRequestConfig returns optimized settings for deterministic output
func DefaultRequestConfig() *RequestConfig {
	return &RequestConfig{
		Model:       "moonshot-v1-8k",
		Temperature: 0.3, // Minimize probability entropy
		MaxTokens:   2048,
		Timeout:     30 * time.Second,
	}
}

// ValidateCommand performs basic security validation
func ValidateCommand(command string) error {
	command = strings.TrimSpace(strings.ToLower(command))

	// Block dangerous patterns
	forbidden := []string{
		"http://", "https://", "ftp://",
		"exec", "system", "shell", "cmd",
		"file://", "javascript:", "data:",
	}

	for _, pattern := range forbidden {
		if strings.Contains(command, pattern) {
			return fmt.Errorf("forbidden pattern detected: %s", pattern)
		}
	}

	if len(command) > 1000 {
		return fmt.Errorf("command too long: %d chars (max 1000)", len(command))
	}

	return nil
}
