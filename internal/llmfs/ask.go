package llmfs

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

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

	// Make the API call
	response, err := f.client.Ask(context.Background(), prompt)
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

func (f *AskFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	f.mu.RLock()
	content := f.lastResponse
	f.mu.RUnlock()
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	s.Length = uint64(len(content))
	return s
}
