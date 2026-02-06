// Package llmfs implements the LLM filesystem exposed via 9P.
package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// NewRoot creates the root directory of the LLM filesystem.
// It takes a Backend which provides access to the LLM.
func NewRoot(client llm.Backend) protocol.Dir {
	root := protocol.NewStaticDir("llm")

	// Core interaction files
	root.AddChild(NewAskFile(client))
	root.AddChild(NewNewFile(client))
	root.AddChild(NewContextFile(client))

	// Settings files
	root.AddChild(NewModelFile(client))
	root.AddChild(NewTemperatureFile(client))
	root.AddChild(NewSystemFile(client))
	root.AddChild(NewThinkingFile(client))
	root.AddChild(NewPrefillFile(client))

	// Token tracking
	root.AddChild(NewTokensFile(client))
	root.AddChild(NewUsageFile(client))
	root.AddChild(NewCompactFile(client))

	// Static files
	root.AddChild(NewExampleFile())

	// Stream directory
	streamDir := protocol.NewStaticDir("stream")
	streamDir.AddChild(NewStreamAskFile(client))
	streamDir.AddChild(NewChunkFile(client))
	root.AddChild(streamDir)

	return root
}
