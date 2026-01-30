package llmfs

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ThinkingFile exposes the thinking token budget (read/write)
// Values: -1 = max (31999), 0 = disabled, >0 = specific budget
// Only effective with CLI backend; API backend ignores this setting.
type ThinkingFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewThinkingFile creates the thinking file
func NewThinkingFile(client llm.Backend) *ThinkingFile {
	return &ThinkingFile{
		BaseFile: protocol.NewBaseFile("thinking", 0666),
		client:   client,
	}
}

func (f *ThinkingFile) Read(p []byte, offset int64) (int, error) {
	tokens := f.client.ThinkingTokens()
	var content string
	switch {
	case tokens < 0:
		content = "max\n"
	case tokens == 0:
		content = "off\n"
	default:
		content = fmt.Sprintf("%d\n", tokens)
	}
	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *ThinkingFile) Write(p []byte, offset int64) (int, error) {
	input := strings.TrimSpace(string(p))
	input = strings.ToLower(input)

	var tokens int
	switch input {
	case "max", "on", "true", "enabled", "-1":
		tokens = -1
	case "off", "false", "disabled", "0":
		tokens = 0
	default:
		var err error
		tokens, err = strconv.Atoi(input)
		if err != nil {
			return 0, fmt.Errorf("invalid thinking value: use 'max', 'off', or a number")
		}
		if tokens < 0 {
			tokens = -1 // Treat any negative as max
		}
	}

	f.client.SetThinkingTokens(tokens)
	return len(p), nil
}

func (f *ThinkingFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	tokens := f.client.ThinkingTokens()
	var content string
	switch {
	case tokens < 0:
		content = "max\n"
	case tokens == 0:
		content = "off\n"
	default:
		content = fmt.Sprintf("%d\n", tokens)
	}
	s.Length = uint64(len(content))
	return s
}
