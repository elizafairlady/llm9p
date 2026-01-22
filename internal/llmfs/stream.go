package llmfs

import (
	"io"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ChunkFile provides streaming access to LLM responses
// Reading blocks until the next chunk is available, then returns it
// Returns EOF when the stream is complete
type ChunkFile struct {
	*protocol.BaseFile
	client *llm.Client
}

// NewChunkFile creates the stream/chunk file
func NewChunkFile(client *llm.Client) *ChunkFile {
	return &ChunkFile{
		BaseFile: protocol.NewBaseFile("chunk", 0444),
		client:   client,
	}
}

func (f *ChunkFile) Read(p []byte, offset int64) (int, error) {
	// If no stream is active, return EOF
	if !f.client.IsStreaming() {
		return 0, io.EOF
	}

	// Block until we get a chunk
	chunk, ok := f.client.ReadStreamChunk()
	if !ok {
		// Stream ended
		return 0, io.EOF
	}

	// Copy the chunk to the buffer
	n := copy(p, chunk)
	return n, nil
}

func (f *ChunkFile) Write(p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

func (f *ChunkFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	// Length is unknown for streaming
	s.Length = 0
	return s
}
