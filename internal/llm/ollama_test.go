package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaClient_BasicOperations(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434")

	// Test default model
	if got := client.Model(); got != "llama3.2" {
		t.Errorf("Model() = %q, want %q", got, "llama3.2")
	}

	// Test SetModel
	client.SetModel("mistral")
	if got := client.Model(); got != "mistral" {
		t.Errorf("Model() after SetModel = %q, want %q", got, "mistral")
	}

	// Test default temperature
	if got := client.Temperature(); got != 0.7 {
		t.Errorf("Temperature() = %v, want %v", got, 0.7)
	}

	// Test SetTemperature
	if err := client.SetTemperature(0.5); err != nil {
		t.Errorf("SetTemperature(0.5) error = %v", err)
	}
	if got := client.Temperature(); got != 0.5 {
		t.Errorf("Temperature() after SetTemperature = %v, want %v", got, 0.5)
	}

	// Test invalid temperature
	if err := client.SetTemperature(3.0); err == nil {
		t.Error("SetTemperature(3.0) should return error")
	}

	// Test SystemPrompt
	client.SetSystemPrompt("You are a helpful assistant")
	if got := client.SystemPrompt(); got != "You are a helpful assistant" {
		t.Errorf("SystemPrompt() = %q, want %q", got, "You are a helpful assistant")
	}

	// Test ThinkingTokens (should be 0 for Ollama)
	if got := client.ThinkingTokens(); got != 0 {
		t.Errorf("ThinkingTokens() = %d, want 0", got)
	}

	// Test Prefill (stored but not used)
	client.SetPrefill("[Assistant] ")
	if got := client.Prefill(); got != "[Assistant] " {
		t.Errorf("Prefill() = %q, want %q", got, "[Assistant] ")
	}

	// Test Reset
	client.Reset()
	if got := len(client.Messages()); got != 0 {
		t.Errorf("Messages() after Reset = %d, want 0", got)
	}
}

func TestOllamaClient_ContextLimitDefaults(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"llama3.2", 8192},
		{"llama2", 4096},
		{"mistral", 8192},
		{"mixtral", 32768},
		{"codellama", 16384},
		{"phi", 2048},
		{"gemma", 8192},
		{"unknown", 4096},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := contextLimitForOllamaModel(tt.model)
			if got != tt.want {
				t.Errorf("contextLimitForOllamaModel(%q) = %d, want %d", tt.model, got, tt.want)
			}
		})
	}
}

func TestOllamaClient_MockServer(t *testing.T) {
	// Create a mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" {
			var req ollamaChatRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			resp := ollamaChatResponse{
				Model:           req.Model,
				Message:         ollamaMessage{Role: "assistant", Content: "Hello! 2+2=4"},
				Done:            true,
				PromptEvalCount: 10,
				EvalCount:       5,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL)

	// Test AskWithHistory
	ctx := context.Background()
	response, tokens, err := client.AskWithHistory(ctx, nil, "What is 2+2?")
	if err != nil {
		t.Fatalf("AskWithHistory() error = %v", err)
	}

	if response != "Hello! 2+2=4" {
		t.Errorf("AskWithHistory() response = %q, want %q", response, "Hello! 2+2=4")
	}

	if tokens != 15 {
		t.Errorf("AskWithHistory() tokens = %d, want %d", tokens, 15)
	}
}

func TestOllamaClient_Messages(t *testing.T) {
	client := NewOllamaClient("")

	// Add system message
	client.AddSystemMessage("You are helpful")

	msgs := client.Messages()
	if len(msgs) != 1 {
		t.Fatalf("Messages() len = %d, want 1", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("Messages()[0].Role = %q, want %q", msgs[0].Role, "system")
	}

	// Test MessagesJSON
	jsonData, err := client.MessagesJSON()
	if err != nil {
		t.Fatalf("MessagesJSON() error = %v", err)
	}
	if len(jsonData) == 0 {
		t.Error("MessagesJSON() returned empty data")
	}
}
