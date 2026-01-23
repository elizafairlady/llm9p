package llmfs

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

// ModelFile exposes the current model name (read/write)
type ModelFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewModelFile creates the model file
func NewModelFile(client llm.Backend) *ModelFile {
	return &ModelFile{
		BaseFile: protocol.NewBaseFile("model", 0666),
		client:   client,
	}
}

func (f *ModelFile) Read(p []byte, offset int64) (int, error) {
	content := f.client.Model() + "\n"
	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *ModelFile) Write(p []byte, offset int64) (int, error) {
	model := strings.TrimSpace(string(p))
	if model == "" {
		return 0, fmt.Errorf("model name cannot be empty")
	}
	f.client.SetModel(model)
	return len(p), nil
}

func (f *ModelFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	s.Length = uint64(len(f.client.Model()) + 1) // +1 for newline
	return s
}

// TemperatureFile exposes the current temperature (read/write)
type TemperatureFile struct {
	*protocol.BaseFile
	client llm.Backend
}

// NewTemperatureFile creates the temperature file
func NewTemperatureFile(client llm.Backend) *TemperatureFile {
	return &TemperatureFile{
		BaseFile: protocol.NewBaseFile("temperature", 0666),
		client:   client,
	}
}

func (f *TemperatureFile) Read(p []byte, offset int64) (int, error) {
	content := fmt.Sprintf("%.2f\n", f.client.Temperature())
	if offset >= int64(len(content)) {
		return 0, io.EOF
	}
	n := copy(p, content[offset:])
	return n, nil
}

func (f *TemperatureFile) Write(p []byte, offset int64) (int, error) {
	tempStr := strings.TrimSpace(string(p))
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid temperature: %w", err)
	}
	if err := f.client.SetTemperature(temp); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (f *TemperatureFile) Stat() protocol.Stat {
	s := f.BaseFile.Stat()
	content := fmt.Sprintf("%.2f\n", f.client.Temperature())
	s.Length = uint64(len(content))
	return s
}
