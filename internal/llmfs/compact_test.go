package llmfs

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestCompactFile_Read_Initial(t *testing.T) {
	mock := NewMockBackend()
	compact := NewCompactFile(mock)

	// Initial read should return "ready"
	buf := make([]byte, 100)
	n, err := compact.Read(buf, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	content := string(buf[:n])
	expected := "ready\n"
	if content != expected {
		t.Errorf("Read() = %q, want %q", content, expected)
	}
}

func TestCompactFile_Write_TriggerCompaction(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 160000
	mock.contextLimit = 200000

	compact := NewCompactFile(mock)

	// Write to trigger compaction
	n, err := compact.Write([]byte("1"), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 1 {
		t.Errorf("Write() n = %d, want 1", n)
	}

	// Verify compaction was called
	if !mock.compactCalled {
		t.Error("Compact() was not called on backend")
	}

	// Read should show "ok: tokens/limit"
	buf := make([]byte, 100)
	readN, _ := compact.Read(buf, 0)
	content := string(buf[:readN])

	if !strings.HasPrefix(content, "ok:") {
		t.Errorf("Read() after compaction = %q, want prefix 'ok:'", content)
	}
}

func TestCompactFile_Write_EmptyNoOp(t *testing.T) {
	mock := NewMockBackend()
	compact := NewCompactFile(mock)

	// Empty write should be no-op
	n, err := compact.Write([]byte(""), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write('') n = %d, want 0", n)
	}

	// Compaction should not be triggered
	if mock.compactCalled {
		t.Error("Compact() should not be called for empty write")
	}
}

func TestCompactFile_Write_WhitespaceNoOp(t *testing.T) {
	mock := NewMockBackend()
	compact := NewCompactFile(mock)

	// Whitespace-only write should be no-op
	_, err := compact.Write([]byte("   \n\t  "), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Compaction should not be triggered
	if mock.compactCalled {
		t.Error("Compact() should not be called for whitespace-only write")
	}
}

func TestCompactFile_Write_Error(t *testing.T) {
	mock := NewMockBackend()
	mock.compactError = fmt.Errorf("compaction failed")

	compact := NewCompactFile(mock)

	// Write should still succeed (error is stored for read)
	n, err := compact.Write([]byte("1"), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 1 {
		t.Errorf("Write() n = %d, want 1", n)
	}

	// Read should show error
	buf := make([]byte, 100)
	readN, _ := compact.Read(buf, 0)
	content := string(buf[:readN])

	if !strings.HasPrefix(content, "error:") {
		t.Errorf("Read() after error = %q, want prefix 'error:'", content)
	}
	if !strings.Contains(content, "compaction failed") {
		t.Errorf("Read() should contain error message, got %q", content)
	}
}

func TestCompactFile_Read_EOF(t *testing.T) {
	mock := NewMockBackend()
	compact := NewCompactFile(mock)

	// Read past end
	buf := make([]byte, 100)
	n, err := compact.Read(buf, 1000)
	if err != io.EOF {
		t.Errorf("Read(offset=1000) error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("Read(offset=1000) n = %d, want 0", n)
	}
}

func TestCompactFile_Stat(t *testing.T) {
	mock := NewMockBackend()
	compact := NewCompactFile(mock)

	stat := compact.Stat()

	// Initial content is "ready\n" = 6 chars
	expected := uint64(6)
	if stat.Length != expected {
		t.Errorf("Stat().Length = %d, want %d", stat.Length, expected)
	}
}

func TestCompactFile_MultipleCompactions(t *testing.T) {
	mock := NewMockBackend()
	mock.totalTokens = 180000
	mock.contextLimit = 200000

	compact := NewCompactFile(mock)

	// First compaction
	compact.Write([]byte("1"), 0)
	if !mock.compactCalled {
		t.Error("First compaction not called")
	}

	// Reset flag
	mock.compactCalled = false
	mock.totalTokens = 100000

	// Second compaction
	compact.Write([]byte("1"), 0)
	if !mock.compactCalled {
		t.Error("Second compaction not called")
	}

	// Check result reflects new token count
	buf := make([]byte, 100)
	readN, _ := compact.Read(buf, 0)
	content := string(buf[:readN])

	if !strings.Contains(content, "25000") { // 100000 / 4 from mock
		t.Errorf("Read() = %q, should contain reduced token count", content)
	}
}
