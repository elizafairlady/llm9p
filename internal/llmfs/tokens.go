package llmfs

import (
	"fmt"
	"io"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// TokensFile exposes the last response token count (read-only)
type TokensFile struct {
	*protocol.BaseFile
	client *llm.Client
}

// NewTokensFile creates the tokens file
func NewTokensFile(client *llm.Client) *TokensFile {
	return &TokensFile{
		BaseFile: protocol.NewBaseFile("tokens", 0444),
		client:   client,
	}
}

func (f *TokensFile) Read(p []byte, offset int64) (int, error) {
	content := fmt.Sprintf("%d\n", f.client.LastTokens())
	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *TokensFile) Write(p []byte, offset int64) (int, error) {
	return 0, protocol.ErrPermission
}

func (f *TokensFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	content := fmt.Sprintf("%d\n", f.client.LastTokens())
	s.Length = uint64(len(content))
	return s
}
