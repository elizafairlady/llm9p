package llmfs

import (
	"io"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// PrefillFile exposes the assistant response prefill (read/write).
// Prefill helps keep the model in character by prepending a string
// to the assistant's response (e.g., "[Veltro] ").
type PrefillFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewPrefillFile creates the prefill file
func NewPrefillFile(client llm.Backend) *PrefillFile {
	return &PrefillFile{
		BaseFile: protocol.NewBaseFile("prefill", 0666),
		client:   client,
	}
}

func (f *PrefillFile) Read(p []byte, offset int64) (int, error) {
	content := f.client.Prefill()
	if content != "" {
		content += "\n"
	}
	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *PrefillFile) Write(p []byte, offset int64) (int, error) {
	prefill := strings.TrimSpace(string(p))
	f.client.SetPrefill(prefill)
	return len(p), nil
}

func (f *PrefillFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	content := f.client.Prefill()
	if content != "" {
		s.Length = uint64(len(content) + 1) // +1 for newline
	} else {
		s.Length = 0
	}
	return s
}
