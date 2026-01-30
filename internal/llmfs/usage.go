package llmfs

import (
	"fmt"
	"io"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// UsageFile provides token usage observability
// Read returns "tokens/limit" (e.g., "45000/200000")
type UsageFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewUsageFile creates the usage file
func NewUsageFile(client llm.Backend) *UsageFile {
	return &UsageFile{
		BaseFile: protocol.NewBaseFile("usage", 0444),
		client:   client,
	}
}

func (f *UsageFile) Read(p []byte, offset int64) (int, error) {
	tokens := f.client.TotalTokens()
	limit := f.client.ContextLimit()
	content := fmt.Sprintf("%d/%d\n", tokens, limit)

	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *UsageFile) Write(p []byte, offset int64) (int, error) {
	return 0, fmt.Errorf("usage is read-only")
}

func (f *UsageFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	tokens := f.client.TotalTokens()
	limit := f.client.ContextLimit()
	content := fmt.Sprintf("%d/%d\n", tokens, limit)
	s.Length = uint64(len(content))
	return s
}
