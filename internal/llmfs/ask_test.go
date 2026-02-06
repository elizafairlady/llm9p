package llmfs

import (
	"io"
	"testing"
)

func TestAskFile_Read_Empty(t *testing.T) {
	mock := NewMockBackend()
	ask := NewAskFile(mock)

	// Initial read should be empty (EOF)
	buf := make([]byte, 100)
	n, err := ask.Read(buf, 0)
	if err != io.EOF {
		t.Errorf("Read() error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("Read() n = %d, want 0", n)
	}
}

func TestAskFile_WriteRead(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "Hello, I'm Claude!"
	ask := NewAskFile(mock)

	// Write a prompt
	prompt := "Hello!"
	n, err := ask.Write([]byte(prompt), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len(prompt) {
		t.Errorf("Write() n = %d, want %d", n, len(prompt))
	}

	// Read response
	buf := make([]byte, 100)
	readN, err := ask.Read(buf, 0)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	response := string(buf[:readN])
	expected := "Hello, I'm Claude!\n"
	if response != expected {
		t.Errorf("Read() = %q, want %q", response, expected)
	}
}

func TestAskFile_Write_EmptyNoOp(t *testing.T) {
	mock := NewMockBackend()
	ask := NewAskFile(mock)

	// Empty write should be no-op
	n, err := ask.Write([]byte(""), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write('') n = %d, want 0", n)
	}

	// Whitespace-only also no-op
	n, err = ask.Write([]byte("   \n\t  "), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
}

func TestAskFile_Write_Error(t *testing.T) {
	mock := NewMockBackend()
	mock.askError = io.ErrUnexpectedEOF
	ask := NewAskFile(mock)

	// Write should succeed (error is stored for read)
	n, err := ask.Write([]byte("test"), 0)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 4 {
		t.Errorf("Write() n = %d, want 4", n)
	}

	// Read should return the error
	buf := make([]byte, 100)
	readN, _ := ask.Read(buf, 0)
	response := string(buf[:readN])

	if len(response) < 6 || response[:6] != "Error:" {
		t.Errorf("Read() = %q, should start with 'Error:'", response)
	}
}

func TestAskFile_Stat(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "Hello!"
	ask := NewAskFile(mock)

	// Stat without response returns 0 length
	stat := ask.Stat()
	if stat.Length != 0 {
		t.Errorf("Stat().Length = %d, want 0 (before write)", stat.Length)
	}

	// Write to get a response
	ask.Write([]byte("test"), 0)

	// Now stat should show response length + newline
	stat = ask.Stat()
	expectedLen := uint64(len("Hello!") + 1) // +1 for newline
	if stat.Length != expectedLen {
		t.Errorf("Stat().Length = %d, want %d", stat.Length, expectedLen)
	}
}

func TestAskFile_ResponseNewline(t *testing.T) {
	mock := NewMockBackend()

	// Response without trailing newline
	mock.askResponse = "No newline"
	ask := NewAskFile(mock)
	ask.Write([]byte("test"), 0)

	buf := make([]byte, 100)
	n, _ := ask.Read(buf, 0)
	response := string(buf[:n])

	// Should have newline added
	if response[len(response)-1] != '\n' {
		t.Errorf("Response should end with newline, got %q", response)
	}

	// Response with trailing newline already
	mock.askResponse = "Has newline\n"
	ask2 := NewAskFile(mock)
	ask2.Write([]byte("test"), 0)

	n, _ = ask2.Read(buf, 0)
	response = string(buf[:n])

	// Should NOT double the newline
	if response != "Has newline\n" {
		t.Errorf("Response = %q, should not double newline", response)
	}
}

func TestCompactThreshold(t *testing.T) {
	// Verify the threshold constant
	if CompactThreshold != 0.80 {
		t.Errorf("CompactThreshold = %f, want 0.80", CompactThreshold)
	}
}
