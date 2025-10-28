/*
Package aiexecutor provides the main AI command execution interface.
Zero-chat, deterministic output with comprehensive error handling.
*/
package aiexecutor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"datacentral-tt/aiconfig"
	"datacentral-tt/persistence"
)

// ExecutorService handles AI command execution
type ExecutorService struct {
	config     *aiconfig.AISystemConfig
	reqConfig  *aiconfig.RequestConfig
	supabase   *persistence.SupabaseClient
	httpClient *http.Client
	logger     *slog.Logger
	apiKey     string
	baseURL    string
}

// NewExecutorService creates a production-ready executor
func NewExecutorService(
	apiKey string,
	supabaseClient *persistence.SupabaseClient,
	logger *slog.Logger,
) *ExecutorService {

	// Production HTTP client with aggressive timeouts for AI APIs
	httpClient := &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			MaxIdleConnsPerHost: 5,
		},
	}

	return &ExecutorService{
		config:     aiconfig.DefaultConfig(),
		reqConfig:  aiconfig.DefaultRequestConfig(),
		supabase:   supabaseClient,
		httpClient: httpClient,
		logger:     logger,
		apiKey:     apiKey,
		baseURL:    "https://api.moonshot.cn/v1/chat/completions",
	}
}

// ExecuteRequest represents a command execution request
type ExecuteRequest struct {
	Command        string `json:"command"`
	PageContent    *string `json:"page_content,omitempty"`
	UserID         string `json:"user_id"`
	IsPrivate      bool `json:"is_private"`
	IdempotencyKey string `json:"idempotency_key,omitempty"` // ULID or UUID v7
}

// ExecuteResponse represents the execution result
type ExecuteResponse struct {
	Content   string    `json:"content"`
	RequestID string    `json:"request_id"`
	Duration  int64     `json:"duration_ms"`
	CreatedAt time.Time `json:"created_at"`
}

// ExecuteAICommand - main entry point for AI command execution
func (es *ExecutorService) ExecuteAICommand(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()
	requestID := fmt.Sprintf("req_%d", startTime.UnixNano())

	// Structured logging for observability
	es.logger.InfoContext(ctx, "executing AI command",
		slog.String("request_id", requestID),
		slog.String("user_id", req.UserID),
		slog.Bool("has_context", req.PageContent != nil),
		slog.Bool("is_private", req.IsPrivate),
	)

	// 1. Validate command
	if err := aiconfig.ValidateCommand(req.Command); err != nil {
		es.logger.WarnContext(ctx, "command validation failed",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	// 2. Build AI prompt with deterministic structure
	messages := es.buildMessages(req.Command, req.PageContent)

	// 3. Call AI with timeout and retries
	aiOutput, err := es.callAI(ctx, messages, requestID)
	if err != nil {
		return nil, fmt.Errorf("AI execution failed: %w", err)
	}

	// 4. Persist result using dual-path logic
	pathType := persistence.PathPublic
	if req.IsPrivate {
		pathType = persistence.PathPrivate
	}

	if err := es.supabase.PersistContent(ctx, aiOutput, req.UserID, pathType); err != nil {
		es.logger.ErrorContext(ctx, "persistence failed",
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		// Don't fail the request if persistence fails - return the content
	}

	duration := time.Since(startTime).Milliseconds()

	es.logger.InfoContext(ctx, "command executed successfully",
		slog.String("request_id", requestID),
		slog.Int64("duration_ms", duration),
		slog.Int("output_length", len(aiOutput)),
	)

	return &ExecuteResponse{
		Content:   aiOutput,
		RequestID: requestID,
		Duration:  duration,
		CreatedAt: startTime,
	}, nil
}

// buildMessages constructs the prompt with immutable system rules
func (es *ExecutorService) buildMessages(command string, pageContent *string) []aiconfig.Message {
	hasContext := pageContent != nil
	messages := es.config.BuildPromptSet(hasContext)

	// Add user message with optional context
	var userContent string
	if hasContext {
		userContent = fmt.Sprintf("CONTEXT:\n%s\nCOMMAND:\n%s", *pageContent, command)
	} else {
		userContent = fmt.Sprintf("COMMAND:\n%s", command)
	}

	messages = append(messages, aiconfig.Message{
		Role:    "user",
		Content: userContent,
	})

	return messages
}

// KimiRequest represents the API request structure
type KimiRequest struct {
	Model       string               `json:"model"`
	Messages    []aiconfig.Message   `json:"messages"`
	Temperature float32              `json:"temperature"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
}

// KimiResponse represents the API response structure
type KimiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// callAI executes the AI request with proper error handling
func (es *ExecutorService) callAI(ctx context.Context, messages []aiconfig.Message, requestID string) (string, error) {
	// Create request payload
	reqPayload := KimiRequest{
		Model:       es.reqConfig.Model,
		Messages:    messages,
		Temperature: es.reqConfig.Temperature,
		MaxTokens:   es.reqConfig.MaxTokens,
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", es.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+es.apiKey)

	// Execute with retry logic
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := es.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed (attempt %d): %w", attempt+1, err)

			es.logger.WarnContext(ctx, "AI API request failed, retrying",
				slog.String("request_id", requestID),
				slog.Int("attempt", attempt+1),
				slog.String("error", err.Error()),
			)

			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("AI API returned HTTP %d (attempt %d)", resp.StatusCode, attempt+1)

			// Don't retry client errors
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				break
			}

			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		// Parse successful response
		var kimiResp KimiResponse
		if err := json.NewDecoder(resp.Body).Decode(&kimiResp); err != nil {
			return "", fmt.Errorf("failed to decode AI response: %w", err)
		}

		if len(kimiResp.Choices) == 0 {
			return "", fmt.Errorf("AI response contained no choices")
		}

		content := strings.TrimSpace(kimiResp.Choices[0].Message.Content)

		// Log usage for monitoring
		es.logger.InfoContext(ctx, "AI request completed",
			slog.String("request_id", requestID),
			slog.Int("prompt_tokens", kimiResp.Usage.PromptTokens),
			slog.Int("completion_tokens", kimiResp.Usage.CompletionTokens),
			slog.Int("total_tokens", kimiResp.Usage.TotalTokens),
		)

		return content, nil
	}

	return "", lastErr
}

// Health check for the AI service
func (es *ExecutorService) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Simple health check command
	req := ExecuteRequest{
		Command:   "Return: OK",
		UserID:    "health_check",
		IsPrivate: true,
	}

	_, err := es.ExecuteAICommand(ctx, req)
	return err
}
