package llmfs

import (
	"context"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// CompactThreshold is the percentage of context limit at which auto-compaction triggers
const CompactThreshold = 0.80

// AskFile is the main interaction file - write a prompt, read the response
type AskFile struct {
	*protocol.BaseFile
	client       llm.Backend
	mu           sync.RWMutex
	lastResponse string
}

// NewAskFile creates the ask file
func NewAskFile(client llm.Backend) *AskFile {
	return &AskFile{
		BaseFile: protocol.NewBaseFile("ask", 0666),
		client:   client,
	}
}

func (f *AskFile) Read(p []byte, offset int64) (int, error) {
	f.mu.RLock()
	content := f.lastResponse
	f.mu.RUnlock()

	// Add newline if not present
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *AskFile) Write(p []byte, offset int64) (int, error) {
	prompt := strings.TrimSpace(string(p))
	if prompt == "" {
		return len(p), nil // Empty write is a no-op
	}

	ctx := context.Background()

	// Check if we need to auto-compact before processing
	tokens := f.client.TotalTokens()
	limit := f.client.ContextLimit()
	threshold := int(float64(limit) * CompactThreshold)

	if tokens > threshold {
		log.Printf("llm9p: auto-compacting at %d/%d tokens (%.0f%% threshold)",
			tokens, limit, CompactThreshold*100)
		if err := f.client.Compact(ctx); err != nil {
			log.Printf("llm9p: auto-compact failed: %v", err)
			// Continue anyway - better to try than to fail
		} else {
			log.Printf("llm9p: auto-compact complete, now at %d tokens",
				f.client.TotalTokens())
		}
	}

	// Make the API call
	response, err := f.client.Ask(ctx, prompt)
	if err != nil {
		// Store error as response so it can be read
		f.mu.Lock()
		f.lastResponse = "Error: " + err.Error()
		f.mu.Unlock()
		return len(p), nil // Return success so client knows write completed
	}

	f.mu.Lock()
	f.lastResponse = response
	f.mu.Unlock()

	return len(p), nil
}

// Stat returns the file's metadata
func (f *AskFile) Stat() protocol.Stat {
	f.mu.RLock()
	length := len(f.lastResponse)
	if length > 0 && !strings.HasSuffix(f.lastResponse, "\n") {
		length++
	}
	f.mu.RUnlock()

	s := f.BaseFile.Stat()
	s.Length = uint64(length)
	return s
}
