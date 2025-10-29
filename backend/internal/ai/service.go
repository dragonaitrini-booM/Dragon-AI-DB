package ai

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type Service struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
	redis      *redis.Client
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

func NewService(apiKey, redisURL string, logger *slog.Logger) *Service {
	// Initialize Redis client
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("failed to parse redis URL", slog.String("error", err.Error()))
		// Fallback to a nil client, caching will be disabled
	}
	redisClient := redis.NewClient(opt)

	return &Service{
		apiKey:  apiKey,
		baseURL: "https://router.huggingface.co/together/v1",
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		logger: logger,
		redis:  redisClient,
	}
}

// System prompts - NEVER expose to client
func (s *Service) getSystemPrompts(hasContext bool) []Message {
	messages := []Message{
		{
			Role: "system",
			Content: `You are "Amor AI," a master data analysis and database expert. Your core directive is to translate natural language commands from users into complex data operations, while presenting the results in a simple, beautiful, and intuitive way. You are the user's guide to their data, making the complex simple.

			**Core Directives:**
			1.  **Expert Persona:** Act as a professional data analyst and database expert. Be confident, knowledgeable, and proactive.
			2.  **Simplify Complexity:** The user does not need to know SQL, database schemas, or data analysis formulas. Your job is to handle all of that behind the scenes.
			3.  **Structured JSON Output:** Always respond with a structured JSON object. The JSON should contain one or more of the following keys: \`chart_spec\`, \`table_data\`, \`insights\`, and \`content\`.
			4.  **Actionable Insights:** Don't just show data; interpret it. Provide an \`insights\` array with 2-3 key takeaways from the analysis.
			5.  **Stunning Visualizations:** For any request that can be visualized, generate a \`chart_spec\` using Chart.js. Make the charts beautiful, with modern color palettes and clear labels.
			6.  **Security First:** Never execute system commands, access external URLs, or perform any action that could compromise security. Only process data analysis and visualization requests.`,
		},
	}

	if hasContext {
		messages = append(messages, Message{
			Role:    "system",
			Content: "The user has provided a data file. Apply their command to this data context. Generate insights, visualizations, and tabular data based *only* on the provided data.",
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

	// Check Redis cache
	cacheKey := s.getCacheKey(req)
	if cached, err := s.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		s.logger.InfoContext(ctx, "cache hit", slog.String("key", cacheKey))
		cached.RequestID = requestID // Assign a new request ID
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
		Model:       "moonshotai/Kimi-K2-Instruct-0905",
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

	// Cache result in Redis
	if err := s.setCache(ctx, cacheKey, result); err != nil {
		s.logger.WarnContext(ctx, "failed to cache result", slog.String("error", err.Error()))
	}

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
	hash := md5.Sum([]byte(req.Command + fmt.Sprintf("%v", req.PageContent)))
	return fmt.Sprintf("cache:%s:%s", req.UserID, hex.EncodeToString(hash[:]))
}

func (s *Service) getFromCache(ctx context.Context, key string) (*ExecuteResponse, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	val, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, err
	}

	var resp ExecuteResponse
	if err := json.Unmarshal(val, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached response: %w", err)
	}

	return &resp, nil
}

func (s *Service) setCache(ctx context.Context, key string, response *ExecuteResponse) error {
	if s.redis == nil {
		return fmt.Errorf("redis client not initialized")
	}

	val, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response for caching: %w", err)
	}

	// Cache expires after 1 hour
	return s.redis.Set(ctx, key, val, 1*time.Hour).Err()
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
