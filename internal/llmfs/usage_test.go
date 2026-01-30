package llmfs

import (
	"io"
	"testing"
)

func TestUsageFile_Read(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 45000
	mock.contextLimit = 200000

	usage := NewUsageFile(mock)

	// Read the full content
	buf := make([]byte, 100)
	n, err := usage.Read(buf, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	content := string(buf[:n])
	expected := "45000/200000\n"
	if content != expected {
		t.Errorf("Read() = %q, want %q", content, expected)
	}
}

func TestUsageFile_Read_Offset(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 1000
	mock.contextLimit = 10000

	usage := NewUsageFile(mock)

	// Read with offset (skip first 5 bytes: "1000/")
	buf := make([]byte, 100)
	n, err := usage.Read(buf, 5)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	content := string(buf[:n])
	expected := "10000\n"
	if content != expected {
		t.Errorf("Read(offset=5) = %q, want %q", content, expected)
	}
}

func TestUsageFile_Read_EOF(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 100
	mock.contextLimit = 1000

	usage := NewUsageFile(mock)

	// Read past end
	buf := make([]byte, 100)
	n, err := usage.Read(buf, 1000)
	if err != io.EOF {
		t.Errorf("Read(offset=1000) error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("Read(offset=1000) n = %d, want 0", n)
	}
}

func TestUsageFile_Write(t *testing.T) {
	mock := NewMockBackend()
	usage := NewUsageFile(mock)

	// Write should fail (read-only)
	n, err := usage.Write([]byte("test"), 0)
	if err == nil {
		t.Error("Write() should return error for read-only file")
	}
	if n != 0 {
		t.Errorf("Write() n = %d, want 0", n)
	}
}

func TestUsageFile_Stat(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 12345
	mock.contextLimit = 200000

	usage := NewUsageFile(mock)
	stat := usage.Stat()

	// Content would be "12345/200000\n" = 14 chars
	expected := uint64(len("12345/200000\n"))
	if stat.Length != expected {
		t.Errorf("Stat().Length = %d, want %d", stat.Length, expected)
	}
}

func TestUsageFile_DynamicContent(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 0
	mock.contextLimit = 100000

	usage := NewUsageFile(mock)

	// First read
	buf := make([]byte, 100)
	n, _ := usage.Read(buf, 0)
	if string(buf[:n]) != "0/100000\n" {
		t.Errorf("First read = %q, want '0/100000\\n'", string(buf[:n]))
	}

	// Update tokens
	mock.totalTokens = 50000

	// Second read should reflect the change
	n, _ = usage.Read(buf, 0)
	if string(buf[:n]) != "50000/100000\n" {
		t.Errorf("Second read = %q, want '50000/100000\\n'", string(buf[:n]))
	}
}
