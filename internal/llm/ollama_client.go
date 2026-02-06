// Ollama backend for local LLM inference.
package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OllamaClient uses the Ollama HTTP API for LLM requests.
type OllamaClient struct {
	mu           sync.RWMutex
	baseURL      string
	httpClient   *http.Client
	model        string
	temperature  float64
	systemPrompt string
	prefill      string // Not supported by Ollama, stored but ignored
	messages     []Message
	lastTokens   int
	totalTokens  int
	streaming    bool
	streamChan   chan string
	streamDone   chan struct{}
}

// ollamaChatRequest represents a request to /api/chat
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

// ollamaMessage represents a message in the Ollama format
type ollamaMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// ollamaOptions represents generation options
type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
}

// ollamaChatResponse represents a response from /api/chat
type ollamaChatResponse struct {
	Model              string        `json:"model"`
	CreatedAt          string        `json:"created_at"`
	Message            ollamaMessage `json:"message"`
	Done               bool          `json:"done"`
	TotalDuration      int64         `json:"total_duration,omitempty"`
	LoadDuration       int64         `json:"load_duration,omitempty"`
	PromptEvalCount    int           `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64         `json:"prompt_eval_duration,omitempty"`
	EvalCount          int           `json:"eval_count,omitempty"`
	EvalDuration       int64         `json:"eval_duration,omitempty"`
}

// ollamaShowResponse represents a response from /api/show
type ollamaShowResponse struct {
	ModelInfo struct {
		ContextLength int `json:"context_length"`
	} `json:"model_info"`
}

// NewOllamaClient creates a new Ollama-based LLM client
func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	// Remove trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &OllamaClient{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: 5 * time.Minute},
		model:       "llama3.2",
		temperature: 0.7,
		messages:    make([]Message, 0),
	}
}

// Model returns the current model name
func (c *OllamaClient) Model() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.model
}

// SetModel sets the model for subsequent requests
func (c *OllamaClient) SetModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.model = model
}

// Temperature returns the current temperature
func (c *OllamaClient) Temperature() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.temperature
}

// SetTemperature sets the temperature for subsequent requests
func (c *OllamaClient) SetTemperature(temp float64) error {
	if temp < 0.0 || temp > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.temperature = temp
	return nil
}

// ThinkingTokens returns 0 - Ollama models don't have extended thinking
func (c *OllamaClient) ThinkingTokens() int {
	return 0
}

// SetThinkingTokens is a no-op for Ollama
func (c *OllamaClient) SetThinkingTokens(tokens int) {
	// No-op: Ollama models don't support extended thinking
}

// Prefill returns the prefill string (not used by Ollama)
func (c *OllamaClient) Prefill() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.prefill
}

// SetPrefill stores the prefill but Ollama doesn't support it
func (c *OllamaClient) SetPrefill(prefill string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prefill = prefill
}

// SystemPrompt returns the current system prompt
func (c *OllamaClient) SystemPrompt() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.systemPrompt
}

// SetSystemPrompt sets the system prompt for subsequent requests
func (c *OllamaClient) SetSystemPrompt(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.systemPrompt = prompt
}

// LastTokens returns the token count from the last response
func (c *OllamaClient) LastTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastTokens
}

// Messages returns a copy of the conversation history
func (c *OllamaClient) Messages() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Message, len(c.messages))
	copy(result, c.messages)
	return result
}

// MessagesJSON returns the conversation history as JSON
func (c *OllamaClient) MessagesJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.MarshalIndent(c.messages, "", "  ")
}

// AddSystemMessage adds a system message to the context
func (c *OllamaClient) AddSystemMessage(content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append([]Message{{Role: "system", Content: content}}, c.messages...)
}

// Reset clears the conversation history
func (c *OllamaClient) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = make([]Message, 0)
	c.lastTokens = 0
	c.totalTokens = 0
}

// TotalTokens returns cumulative token count for this conversation
func (c *OllamaClient) TotalTokens() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalTokens
}

// ContextLimit returns the model's context window limit
func (c *OllamaClient) ContextLimit() int {
	c.mu.RLock()
	model := c.model
	c.mu.RUnlock()

	// Try to get from Ollama API
	limit := c.queryContextLimit(model)
	if limit > 0 {
		return limit
	}

	// Fallback defaults for common models
	return contextLimitForOllamaModel(model)
}

// queryContextLimit queries the Ollama API for model context length
func (c *OllamaClient) queryContextLimit(model string) int {
	req, err := http.NewRequest("POST", c.baseURL+"/api/show", bytes.NewBufferString(fmt.Sprintf(`{"name":"%s"}`, model)))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var showResp ollamaShowResponse
	if err := json.NewDecoder(resp.Body).Decode(&showResp); err != nil {
		return 0
	}

	return showResp.ModelInfo.ContextLength
}

// contextLimitForOllamaModel returns default context limits for known models
func contextLimitForOllamaModel(model string) int {
	model = strings.ToLower(model)
	switch {
	case strings.Contains(model, "llama3"):
		return 8192
	case strings.Contains(model, "llama2"):
		return 4096
	case strings.Contains(model, "mistral"):
		return 8192
	case strings.Contains(model, "mixtral"):
		return 32768
	case strings.Contains(model, "codellama"):
		return 16384
	case strings.Contains(model, "phi"):
		return 2048
	case strings.Contains(model, "gemma"):
		return 8192
	default:
		return 4096 // Conservative default
	}
}

// buildOllamaMessages converts internal messages to Ollama format
func (c *OllamaClient) buildOllamaMessages(history []Message, prompt string) []ollamaMessage {
	var msgs []ollamaMessage

	// Add system prompt first if set
	if c.systemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: c.systemPrompt})
	}

	// Add history, handling our system messages
	for _, msg := range history {
		switch msg.Role {
		case "system":
			msgs = append(msgs, ollamaMessage{Role: "system", Content: msg.Content})
		case "user":
			msgs = append(msgs, ollamaMessage{Role: "user", Content: msg.Content})
		case "assistant":
			msgs = append(msgs, ollamaMessage{Role: "assistant", Content: msg.Content})
		}
	}

	// Add the new user prompt
	msgs = append(msgs, ollamaMessage{Role: "user", Content: prompt})

	return msgs
}

// Compact summarizes the conversation to reduce token usage
func (c *OllamaClient) Compact(ctx context.Context) error {
	c.mu.Lock()
	if len(c.messages) < 4 {
		c.mu.Unlock()
		return nil // Not enough to compact
	}

	// Build conversation text for summarization
	var conversationText string
	for _, msg := range c.messages {
		if msg.Role == "system" {
			continue
		}
		conversationText += fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content)
	}

	model := c.model
	c.mu.Unlock()

	// Use Ollama to summarize
	summaryPrompt := "Summarize this conversation concisely, preserving key facts, decisions, and context needed to continue:\n\n" + conversationText

	req := ollamaChatRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "user", Content: summaryPrompt},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("compaction failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return fmt.Errorf("compaction parse failed: %w", err)
	}

	summary := chatResp.Message.Content

	// Replace conversation with summary
	c.mu.Lock()
	c.messages = []Message{{Role: "system", Content: "Previous conversation summary: " + summary}}
	c.totalTokens = chatResp.PromptEvalCount + chatResp.EvalCount
	c.mu.Unlock()

	return nil
}

// Ask sends a prompt to Ollama and returns the response
func (c *OllamaClient) Ask(ctx context.Context, prompt string) (string, error) {
	c.mu.Lock()
	c.messages = append(c.messages, Message{Role: "user", Content: prompt})
	msgs := c.buildOllamaMessages(c.messages[:len(c.messages)-1], prompt) // Don't include the just-added msg
	model := c.model
	temp := c.temperature
	c.mu.Unlock()

	req := ollamaChatRequest{
		Model:    model,
		Messages: msgs,
		Stream:   false,
		Options:  &ollamaOptions{Temperature: temp},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		c.removeLastMessage()
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		c.removeLastMessage()
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		c.removeLastMessage()
		return "", fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.removeLastMessage()
		return "", fmt.Errorf("Ollama API error: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		c.removeLastMessage()
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	responseText := chatResp.Message.Content

	// Update state
	c.mu.Lock()
	c.messages = append(c.messages, Message{Role: "assistant", Content: responseText})
	c.lastTokens = chatResp.PromptEvalCount + chatResp.EvalCount
	c.totalTokens += c.lastTokens
	c.mu.Unlock()

	// Record metrics
	RecordMetrics(chatResp.PromptEvalCount, chatResp.EvalCount, latencyMs)

	return responseText, nil
}

// removeLastMessage removes the last message from history (used on error)
func (c *OllamaClient) removeLastMessage() {
	c.mu.Lock()
	if len(c.messages) > 0 {
		c.messages = c.messages[:len(c.messages)-1]
	}
	c.mu.Unlock()
}

// AskWithHistory sends a prompt with explicit message history for per-fid isolation.
func (c *OllamaClient) AskWithHistory(ctx context.Context, history []Message, prompt string) (string, int, error) {
	c.mu.RLock()
	model := c.model
	temp := c.temperature
	systemPrompt := c.systemPrompt
	c.mu.RUnlock()

	// Build Ollama messages
	var msgs []ollamaMessage

	// Add system prompt first if set
	if systemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: systemPrompt})
	}

	// Add history
	for _, msg := range history {
		switch msg.Role {
		case "system":
			msgs = append(msgs, ollamaMessage{Role: "system", Content: msg.Content})
		case "user":
			msgs = append(msgs, ollamaMessage{Role: "user", Content: msg.Content})
		case "assistant":
			msgs = append(msgs, ollamaMessage{Role: "assistant", Content: msg.Content})
		}
	}

	// Add the new user prompt
	msgs = append(msgs, ollamaMessage{Role: "user", Content: prompt})

	req := ollamaChatRequest{
		Model:    model,
		Messages: msgs,
		Stream:   false,
		Options:  &ollamaOptions{Temperature: temp},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		return "", 0, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("Ollama API error: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", 0, fmt.Errorf("failed to decode response: %w", err)
	}

	responseText := chatResp.Message.Content
	tokens := chatResp.PromptEvalCount + chatResp.EvalCount

	// Record metrics
	RecordMetrics(chatResp.PromptEvalCount, chatResp.EvalCount, latencyMs)

	return responseText, tokens, nil
}

// StartStream begins streaming a response for the given prompt
func (c *OllamaClient) StartStream(ctx context.Context, prompt string) error {
	c.mu.Lock()
	if c.streaming {
		c.mu.Unlock()
		return fmt.Errorf("stream already in progress")
	}

	c.messages = append(c.messages, Message{Role: "user", Content: prompt})
	msgs := c.buildOllamaMessages(c.messages[:len(c.messages)-1], prompt)
	model := c.model
	temp := c.temperature

	c.streaming = true
	c.streamChan = make(chan string, 100)
	c.streamDone = make(chan struct{})
	c.mu.Unlock()

	go func() {
		var fullResponse string
		var promptTokens, evalTokens int

		defer func() {
			c.mu.Lock()
			if fullResponse != "" {
				c.messages = append(c.messages, Message{Role: "assistant", Content: fullResponse})
				c.lastTokens = promptTokens + evalTokens
				c.totalTokens += c.lastTokens
			}
			c.streaming = false
			close(c.streamChan)
			close(c.streamDone)
			c.mu.Unlock()
		}()

		req := ollamaChatRequest{
			Model:    model,
			Messages: msgs,
			Stream:   true,
			Options:  &ollamaOptions{Temperature: temp},
		}

		reqBody, err := json.Marshal(req)
		if err != nil {
			select {
			case c.streamChan <- fmt.Sprintf("[Error: %v]", err):
			case <-ctx.Done():
			}
			c.removeLastMessage()
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqBody))
		if err != nil {
			select {
			case c.streamChan <- fmt.Sprintf("[Error: %v]", err):
			case <-ctx.Done():
			}
			c.removeLastMessage()
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			select {
			case c.streamChan <- fmt.Sprintf("[Error: %v]", err):
			case <-ctx.Done():
			}
			c.removeLastMessage()
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			select {
			case c.streamChan <- fmt.Sprintf("[Error: HTTP %d: %s]", resp.StatusCode, string(body)):
			case <-ctx.Done():
			}
			c.removeLastMessage()
			return
		}

		// Read streaming NDJSON responses
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var chatResp ollamaChatResponse
			if err := json.Unmarshal([]byte(line), &chatResp); err != nil {
				continue
			}

			// Send content chunk
			if chatResp.Message.Content != "" {
				fullResponse += chatResp.Message.Content
				select {
				case c.streamChan <- chatResp.Message.Content:
				case <-ctx.Done():
					return
				}
			}

			// Capture final token counts
			if chatResp.Done {
				promptTokens = chatResp.PromptEvalCount
				evalTokens = chatResp.EvalCount
			}
		}

		if err := scanner.Err(); err != nil {
			if fullResponse == "" {
				select {
				case c.streamChan <- fmt.Sprintf("[Error: %v]", err):
				case <-ctx.Done():
				}
				c.removeLastMessage()
			}
		}
	}()

	return nil
}

// ReadStreamChunk reads the next chunk from the stream
func (c *OllamaClient) ReadStreamChunk() (string, bool) {
	c.mu.RLock()
	streamChan := c.streamChan
	c.mu.RUnlock()

	if streamChan == nil {
		return "", false
	}

	chunk, ok := <-streamChan
	return chunk, ok
}

// IsStreaming returns whether a stream is currently in progress
func (c *OllamaClient) IsStreaming() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.streaming
}

// WaitStream waits for the current stream to complete
func (c *OllamaClient) WaitStream() {
	c.mu.RLock()
	done := c.streamDone
	c.mu.RUnlock()

	if done != nil {
		<-done
	}
}
