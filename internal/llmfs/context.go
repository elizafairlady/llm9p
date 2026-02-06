package llmfs

import (
	"io"
	"strings"
	"sync"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ContextFile exposes the conversation history.
// Read: returns JSON of conversation history
// Write: appends a system message to the context
type ContextFile struct {
	*protocol.BaseFile
	client llm.Backend
	mu     sync.RWMutex
}

// NewContextFile creates the context file
func NewContextFile(client llm.Backend) *ContextFile {
	return &ContextFile{
		BaseFile: protocol.NewBaseFile("context", 0666),
		client:   client,
	}
}

// Read implements File.Read
func (f *ContextFile) Read(p []byte, offset int64) (int, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	content, err := f.client.MessagesJSON()
	if err != nil {
		return 0, err
	}
	// Add newline
	content = append(content, '\n')

	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

// Write implements File.Write - appends a system message to context
func (f *ContextFile) Write(p []byte, offset int64) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		f.mu.Lock()
		f.client.AddSystemMessage(msg)
		f.mu.Unlock()
	}
	return len(p), nil
}

// Stat returns the file's metadata
func (f *ContextFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	// Length is dynamic
	s.Length = 0
	return s
}
