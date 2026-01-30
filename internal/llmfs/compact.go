package llmfs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// CompactFile allows manual compaction trigger
// Write anything to trigger compaction
// Read returns status ("ok" or "error: ...")
type CompactFile struct {
	*protocol.BaseFile
	client     llm.Backend
	mu         sync.RWMutex
	lastResult string
}

// NewCompactFile creates the compact file
func NewCompactFile(client llm.Backend) *CompactFile {
	return &CompactFile{
		BaseFile:   protocol.NewBaseFile("compact", 0666),
		client:     client,
		lastResult: "ready\n",
	}
}

func (f *CompactFile) Read(p []byte, offset int64) (int, error) {
	f.mu.RLock()
	content := f.lastResult
	f.mu.RUnlock()

	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *CompactFile) Write(p []byte, offset int64) (int, error) {
	cmd := strings.TrimSpace(string(p))
	if cmd == "" {
		return len(p), nil
	}

	// Trigger compaction
	err := f.client.Compact(context.Background())

	f.mu.Lock()
	if err != nil {
		f.lastResult = fmt.Sprintf("error: %v\n", err)
	} else {
		tokens := f.client.TotalTokens()
		limit := f.client.ContextLimit()
		f.lastResult = fmt.Sprintf("ok: %d/%d\n", tokens, limit)
	}
	f.mu.Unlock()

	return len(p), nil
}

func (f *CompactFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	f.mu.RLock()
	s.Length = uint64(len(f.lastResult))
	f.mu.RUnlock()
	return s
}
