package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Service struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
	cache      map[string]*CacheEntry
	cacheMutex sync.RWMutex
}

type CacheEntry struct {
	Response  *ExecuteResponse
	CreatedAt time.Time
}

type ExecuteRequest struct {
	Command     string  `json:"command"`
	PageContent *string `json:"page_content,omitempty"`
	UserID      string  `json:"user_id"`
	IsPrivate   bool    `json:"is_private"`
	Context     *string `json:"context,omitempty"`
}

type ExecuteResponse struct {
	Content     string            `json:"content"`
	RequestID   string            `json:"request_id"`
	Duration    int64             `json:"duration_ms"`
	CreatedAt   time.Time         `json:"created_at"`
	TokensUsed  int               `json:"tokens_used"`
	ChartSpec   map[string]any    `json:"chart_spec,omitempty"`
	TableData   []map[string]any  `json:"table_data,omitempty"`
	Insights    []string          `json:"insights,omitempty"`
}

type BatchRequest struct {
	Requests []ExecuteRequest `json:"requests"`
}

type BatchResponse struct {
	Results []ExecuteResponse `json:"results"`
	Errors  []string          `json:"errors,omitempty"`
}

func NewService(apiKey string, logger *slog.Logger) *Service {
	return &Service{
		apiKey:  apiKey,
		baseURL: "https://api.moonshot.cn/v1/chat/completions",
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		logger: logger,
		cache:  make(map[string]*CacheEntry),
	}
}

// System prompts - NEVER expose to client
func (s *Service) getSystemPrompts(hasContext bool) []Message {
	messages := []Message{
		{
			Role: "system",
			Content: `You are a professional data analysis AI. Generate structured responses in JSON format.
			For visualizations, include chart_spec with Chart.js configuration.
			For data analysis, include insights array with key findings.
			Be concise and actionable. Never include explanatory text outside the JSON structure.`,
		},
		{
			Role: "system",
			Content: `Security rules:
			- Only process data analysis and visualization requests
			- Never execute system commands or access external URLs
			- Validate all data inputs for safety
			- Return structured data only`,
		},
	}

	if hasContext {
		messages = append(messages, Message{
			Role:    "system",
			Content: "Apply the user's command to the provided data context. Generate insights based on the actual data provided.",
		})
	}

	return messages
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type KimiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type KimiResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *Service) ExecuteCommand(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()
	requestID := fmt.Sprintf("req_%d", startTime.UnixNano())

	// Check cache
	cacheKey := s.getCacheKey(req)
	if cached := s.getFromCache(cacheKey); cached != nil {
		return cached, nil
	}

	// Validate command
	if err := s.validateCommand(req.Command); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	// Build messages
	messages := s.getSystemPrompts(req.PageContent != nil)

	var userContent string
	if req.PageContent != nil {
		userContent = fmt.Sprintf("DATA CONTEXT:\n%s\n\nCOMMAND: %s", *req.PageContent, req.Command)
	} else {
		userContent = req.Command
	}

	messages = append(messages, Message{
		Role:    "user",
		Content: userContent,
	})

	// Call AI API
	kimiReq := KimiRequest{
		Model:       "moonshot-v1-8k",
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	response, err := s.callKimiAPI(ctx, kimiReq)
	if err != nil {
		return nil, fmt.Errorf("AI API call failed: %w", err)
	}

	// Parse response
	result := &ExecuteResponse{
		RequestID:  requestID,
		Duration:   time.Since(startTime).Milliseconds(),
		CreatedAt:  startTime,
		TokensUsed: response.Usage.TotalTokens,
	}

	// Try to parse as JSON first
	var jsonResp map[string]any
	content := strings.TrimSpace(response.Choices[0].Message.Content)

	if err := json.Unmarshal([]byte(content), &jsonResp); err == nil {
		// Structured response
		if chartSpec, ok := jsonResp["chart_spec"].(map[string]any); ok {
			result.ChartSpec = s.optimizeChartSpec(chartSpec)
		}
		if tableData, ok := jsonResp["table_data"].([]any); ok {
			result.TableData = s.convertToMapSlice(tableData)
		}
		if insights, ok := jsonResp["insights"].([]any); ok {
			result.Insights = s.convertToStringSlice(insights)
		}
		if contentStr, ok := jsonResp["content"].(string); ok {
			result.Content = contentStr
		} else {
			result.Content = content
		}
	} else {
		// Plain text response
		result.Content = content
	}

	// Cache result
	s.setCache(cacheKey, result)

	return result, nil
}

func (s *Service) ExecuteBatch(ctx context.Context, req BatchRequest) (*BatchResponse, error) {
	response := &BatchResponse{
		Results: make([]ExecuteResponse, 0, len(req.Requests)),
		Errors:  make([]string, 0),
	}

	// Process in parallel with concurrency limit
	const maxConcurrency = 5
	semaphore := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	resultChan := make(chan struct{
		index  int
		result *ExecuteResponse
		err    error
	}, len(req.Requests))

	for i, execReq := range req.Requests {
		wg.Add(1)
		go func(index int, request ExecuteRequest) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := s.ExecuteCommand(ctx, request)
			resultChan <- struct{
				index  int
				result *ExecuteResponse
				err    error
			}{index, result, err}
		}(i, execReq)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results in order
	results := make([]*ExecuteResponse, len(req.Requests))
	errors := make([]error, len(req.Requests))

	for result := range resultChan {
		results[result.index] = result.result
		errors[result.index] = result.err
	}

	for i, result := range results {
		if errors[i] != nil {
			response.Errors = append(response.Errors, errors[i].Error())
			response.Results = append(response.Results, ExecuteResponse{
				Content:   "Error processing request",
				RequestID: fmt.Sprintf("error_%d", i),
				CreatedAt: time.Now(),
			})
		} else {
			response.Results = append(response.Results, *result)
		}
	}

	return response, nil
}

func (s *Service) callKimiAPI(ctx context.Context, req KimiRequest) (*KimiResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var kimiResp KimiResponse
	if err := json.NewDecoder(resp.Body).Decode(&kimiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(kimiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &kimiResp, nil
}

func (s *Service) optimizeChartSpec(spec map[string]any) map[string]any {
	// Add Chart.js performance optimizations
	if options, ok := spec["options"].(map[string]any); ok {
		// Add responsive options
		options["responsive"] = true
		options["maintainAspectRatio"] = false

		// Add animation settings
		options["animation"] = map[string]any{
			"duration": 750,
			"easing":   "easeInOutQuart",
		}

		// Optimize for large datasets
		if data, ok := spec["data"].(map[string]any); ok {
			if datasets, ok := data["datasets"].([]any); ok && len(datasets) > 0 {
				if dataset, ok := datasets[0].(map[string]any); ok {
					if dataPoints, ok := dataset["data"].([]any); ok {
						pointCount := len(dataPoints)

						if pointCount > 1000 {
							options["elements"] = map[string]any{
								"point": map[string]any{"radius": 0},
							}
							options["parsing"] = false
						}

						if pointCount > 5000 {
							options["decimation"] = map[string]any{
								"enabled":   true,
								"algorithm": "lttb",
								"samples":   1000,
							}
						}
					}
				}
			}
		}
	}

	return spec
}

func (s *Service) validateCommand(command string) error {
	command = strings.TrimSpace(strings.ToLower(command))

	if len(command) == 0 {
		return fmt.Errorf("empty command")
	}

	if len(command) > 2000 {
		return fmt.Errorf("command too long: %d characters", len(command))
	}

	// Security patterns
	forbidden := []string{
		"exec", "system", "shell", "cmd",
		"http://", "https://", "file://",
		"javascript:", "data:", "vbscript:",
		"<script", "eval(", "setTimeout",
	}

	for _, pattern := range forbidden {
		if strings.Contains(command, pattern) {
			return fmt.Errorf("forbidden pattern detected")
		}
	}

	return nil
}

func (s *Service) getCacheKey(req ExecuteRequest) string {
	return fmt.Sprintf("%s_%s_%v", req.UserID, req.Command, req.PageContent != nil)
}

func (s *Service) getFromCache(key string) *ExecuteResponse {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	entry, exists := s.cache[key]
	if !exists {
		return nil
	}

	// Cache expires after 10 minutes
	if time.Since(entry.CreatedAt) > 10*time.Minute {
		delete(s.cache, key)
		return nil
	}

	return entry.Response
}

func (s *Service) setCache(key string, response *ExecuteResponse) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.cache[key] = &CacheEntry{
		Response:  response,
		CreatedAt: time.Now(),
	}
}

func (s *Service) convertToMapSlice(data []any) []map[string]any {
	result := make([]map[string]any, len(data))
	for i, item := range data {
		if m, ok := item.(map[string]any); ok {
			result[i] = m
		}
	}
	return result
}

func (s *Service) convertToStringSlice(data []any) []string {
	result := make([]string, len(data))
	for i, item := range data {
		if s, ok := item.(string); ok {
			result[i] = s
		}
	}
	return result
}
