package llmfs

import (
	"context"
	"io"
	"log"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// CompactThreshold is the percentage of context limit at which auto-compaction triggers
const CompactThreshold = 0.80

// AskFile is the main interaction file - write a prompt, read the response.
// It implements FidAwareFile to provide per-fid session isolation.
type AskFile struct {
	*protocol.BaseFile
	sm *llm.SessionManager
}

// NewAskFile creates the ask file
func NewAskFile(sm *llm.SessionManager) *AskFile {
	return &AskFile{
		BaseFile: protocol.NewBaseFile("ask", 0666),
		sm:       sm,
	}
}

// Read implements File.Read (fallback for non-fid-aware access)
func (f *AskFile) Read(p []byte, offset int64) (int, error) {
	// Without fid context, we can't return session-specific data
	// Return empty to indicate no data available
	return 0, io.EOF
}

// Write implements File.Write (fallback for non-fid-aware access)
func (f *AskFile) Write(p []byte, offset int64) (int, error) {
	// Without fid context, we can't process the request properly
	return 0, protocol.ErrPermission
}

// ReadFid implements FidAwareFile.ReadFid
func (f *AskFile) ReadFid(fid uint32, p []byte, offset int64) (int, error) {
	session := f.sm.GetOrCreate(fid)
	content := session.LastResponse()

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

// WriteFid implements FidAwareFile.WriteFid
func (f *AskFile) WriteFid(fid uint32, p []byte, offset int64) (int, error) {
	prompt := strings.TrimSpace(string(p))
	if prompt == "" {
		return len(p), nil // Empty write is a no-op
	}

	ctx := context.Background()

	// Check if we need to auto-compact before processing
	session := f.sm.GetOrCreate(fid)
	tokens := session.TotalTokens()
	limit := f.sm.ContextLimit()
	threshold := int(float64(limit) * CompactThreshold)

	if tokens > threshold {
		log.Printf("llm9p: fid %d at %d/%d tokens (%.0f%% threshold) - consider resetting",
			fid, tokens, limit, CompactThreshold*100)
		// Note: Per-session compaction would require a different approach
		// For now, just log a warning. Session reset via /new is the solution.
	}

	// Make the API call using the session
	_, err := f.sm.Ask(ctx, fid, prompt)
	if err != nil {
		// Error is stored in session.LastResponse by SessionManager
		return len(p), nil // Return success so client knows write completed
	}

	return len(p), nil
}

// CloseFid implements FidAwareFile.CloseFid
func (f *AskFile) CloseFid(fid uint32) error {
	// Clean up the session when the fid is clunked
	f.sm.Remove(fid)
	return nil
}

// Stat returns the file's metadata
func (f *AskFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	// Length is dynamic based on session, but without fid context we return 0
	s.Length = 0
	return s
}
