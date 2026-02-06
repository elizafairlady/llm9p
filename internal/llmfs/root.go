// Package llmfs implements the LLM filesystem exposed via 9P.
package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// NewRoot creates the root directory of the LLM filesystem.
// It takes a SessionManager which provides per-fid session isolation
// and access to the underlying backend for global settings.
func NewRoot(sm *llm.SessionManager) protocol.Dir {
	backend := sm.Backend()
	root := protocol.NewStaticDir("llm")

	// Session-aware files (per-fid isolation)
	root.AddChild(NewAskFile(sm))
	root.AddChild(NewNewFile(sm))
	root.AddChild(NewContextFile(sm))

	// Global settings files (shared across all fids)
	root.AddChild(NewModelFile(backend))
	root.AddChild(NewTemperatureFile(backend))
	root.AddChild(NewSystemFile(backend))
	root.AddChild(NewThinkingFile(backend))
	root.AddChild(NewPrefillFile(backend))

	// Token tracking (uses backend's global counters)
	root.AddChild(NewTokensFile(backend))
	root.AddChild(NewUsageFile(backend))
	root.AddChild(NewCompactFile(backend))

	// Static files
	root.AddChild(NewExampleFile())

	// Add stream directory (uses backend directly)
	streamDir := protocol.NewStaticDir("stream")
	streamDir.AddChild(NewStreamAskFile(backend))
	streamDir.AddChild(NewChunkFile(backend))
	root.AddChild(streamDir)

	return root
}
