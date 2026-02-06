package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// NewFile is a write-only file that resets the conversation when written to.
type NewFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewNewFile creates the new file
func NewNewFile(client llm.Backend) *NewFile {
	return &NewFile{
		BaseFile: protocol.NewBaseFile("new", 0222),
		client:   client,
	}
}

// Read implements File.Read
func (f *NewFile) Read(p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

// Write implements File.Write - writing anything resets the conversation
func (f *NewFile) Write(p []byte, offset int64) (int, error) {
	f.client.Reset()
	return len(p), nil
}

// Stat returns the file's metadata
func (f *NewFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	s.Length = 0
	return s
}
