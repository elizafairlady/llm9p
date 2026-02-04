package llmfs

import (
	"io"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ContextFile exposes the conversation history.
// It implements FidAwareFile to provide per-fid session isolation.
// Read: returns JSON of conversation history for this fid's session
// Write: appends a system message to this fid's session context
type ContextFile struct {
	*protocol.BaseFile
	sm *llm.SessionManager
}

// NewContextFile creates the context file
func NewContextFile(sm *llm.SessionManager) *ContextFile {
	return &ContextFile{
		BaseFile: protocol.NewBaseFile("context", 0666),
		sm:       sm,
	}
}

// Read implements File.Read (fallback for non-fid-aware access)
func (f *ContextFile) Read(p []byte, offset int64) (int, error) {
	// Without fid context, return empty
	return 0, io.EOF
}

// Write implements File.Write (fallback for non-fid-aware access)
func (f *ContextFile) Write(p []byte, offset int64) (int, error) {
	// Without fid context, we can't add to a specific session
	return 0, protocol.ErrPermission
}

// ReadFid implements FidAwareFile.ReadFid
func (f *ContextFile) ReadFid(fid uint32, p []byte, offset int64) (int, error) {
	session := f.sm.GetOrCreate(fid)
	content, err := session.MessagesJSON()
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

// WriteFid implements FidAwareFile.WriteFid
func (f *ContextFile) WriteFid(fid uint32, p []byte, offset int64) (int, error) {
	// Writing appends a system message to this fid's session context
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		session := f.sm.GetOrCreate(fid)
		session.AddSystemMessage(msg)
	}
	return len(p), nil
}

// CloseFid implements FidAwareFile.CloseFid
func (f *ContextFile) CloseFid(fid uint32) error {
	// No per-fid state to clean up for this file
	// (session cleanup is handled by AskFile)
	return nil
}

// Stat returns the file's metadata
func (f *ContextFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	// Length is dynamic based on session, but without fid context we return 0
	s.Length = 0
	return s
}
