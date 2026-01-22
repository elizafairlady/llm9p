package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// NewFile is a write-only file that resets the conversation when written to
type NewFile struct {
	*protocol.BaseFile
	client *llm.Client
}

// NewNewFile creates the new file
func NewNewFile(client *llm.Client) *NewFile {
	return &NewFile{
		BaseFile: protocol.NewBaseFile("new", 0222),
		client:   client,
	}
}

func (f *NewFile) Read(p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

func (f *NewFile) Write(p []byte, offset int64) (int, error) {
	// Any write resets the conversation
	f.client.Reset()
	return len(p), nil
}

func (f *NewFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	s.Length = 0
	return s
}
