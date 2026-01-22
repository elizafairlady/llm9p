package llmfs

import (
	"io"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ContextFile exposes the conversation history
// Read: returns JSON of conversation history
// Write: appends a system message to context
type ContextFile struct {
	*protocol.BaseFile
	client *llm.Client
}

// NewContextFile creates the context file
func NewContextFile(client *llm.Client) *ContextFile {
	return &ContextFile{
		BaseFile: protocol.NewBaseFile("context", 0666),
		client:   client,
	}
}

func (f *ContextFile) Read(p []byte, offset int64) (int, error) {
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

func (f *ContextFile) Write(p []byte, offset int64) (int, error) {
	// Writing appends a system message to the context
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		f.client.AddSystemMessage(msg)
	}
	return len(p), nil
}

func (f *ContextFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	content, _ := f.client.MessagesJSON()
	s.Length = uint64(len(content) + 1) // +1 for newline
	return s
}
