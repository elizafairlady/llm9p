package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// NewFile is a write-only file that resets the conversation when written to.
// It implements FidAwareFile to reset only the session for the writing fid.
type NewFile struct {
	*protocol.BaseFile
	sm *llm.SessionManager
}

// NewNewFile creates the new file
func NewNewFile(sm *llm.SessionManager) *NewFile {
	return &NewFile{
		BaseFile: protocol.NewBaseFile("new", 0222),
		sm:       sm,
	}
}

// Read implements File.Read
func (f *NewFile) Read(p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

// Write implements File.Write (fallback for non-fid-aware access)
func (f *NewFile) Write(p []byte, offset int64) (int, error) {
	// Without fid context, we can't reset a specific session
	return 0, protocol.ErrPermission
}

// ReadFid implements FidAwareFile.ReadFid
func (f *NewFile) ReadFid(fid uint32, p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

// WriteFid implements FidAwareFile.WriteFid
func (f *NewFile) WriteFid(fid uint32, p []byte, offset int64) (int, error) {
	// Reset only the session for this fid
	f.sm.Reset(fid)
	return len(p), nil
}

// CloseFid implements FidAwareFile.CloseFid
func (f *NewFile) CloseFid(fid uint32) error {
	// No per-fid state to clean up for this file
	return nil
}

// Stat returns the file's metadata
func (f *NewFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	s.Length = 0
	return s
}
