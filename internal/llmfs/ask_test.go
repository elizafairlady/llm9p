package llmfs

import (
	"io"
	"testing"

	"github.com/NERVsystems/llm9p/internal/llm"
)

func TestAskFile_Read_Empty(t *testing.T) {
	mock := NewMockBackend()
	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	// Initial read with fid should be empty (EOF)
	buf := make([]byte, 100)
	n, err := ask.ReadFid(1, buf, 0)
	if err != io.EOF {
		t.Errorf("ReadFid() error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("ReadFid() n = %d, want 0", n)
	}
}

func TestAskFile_WriteRead(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "Hello, I'm Claude!"

	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	fid := uint32(1)

	// Write a prompt
	prompt := "Hello!"
	n, err := ask.WriteFid(fid, []byte(prompt), 0)
	if err != nil {
		t.Fatalf("WriteFid() error: %v", err)
	}
	if n != len(prompt) {
		t.Errorf("WriteFid() n = %d, want %d", n, len(prompt))
	}

	// Read response
	buf := make([]byte, 100)
	readN, err := ask.ReadFid(fid, buf, 0)
	if err != nil {
		t.Fatalf("ReadFid() error: %v", err)
	}

	response := string(buf[:readN])
	expected := "Hello, I'm Claude!\n"
	if response != expected {
		t.Errorf("ReadFid() = %q, want %q", response, expected)
	}
}

func TestAskFile_Write_EmptyNoOp(t *testing.T) {
	mock := NewMockBackend()
	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	fid := uint32(1)

	// Empty write should be no-op
	n, err := ask.WriteFid(fid, []byte(""), 0)
	if err != nil {
		t.Fatalf("WriteFid() error: %v", err)
	}
	if n != 0 {
		t.Errorf("WriteFid('') n = %d, want 0", n)
	}

	// Whitespace-only also no-op
	n, err = ask.WriteFid(fid, []byte("   \n\t  "), 0)
	if err != nil {
		t.Fatalf("WriteFid() error: %v", err)
	}
}

func TestAskFile_Write_Error(t *testing.T) {
	mock := NewMockBackend()
	mock.askError = io.ErrUnexpectedEOF

	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	fid := uint32(1)

	// Write should succeed (error is stored for read)
	n, err := ask.WriteFid(fid, []byte("test"), 0)
	if err != nil {
		t.Fatalf("WriteFid() error: %v", err)
	}
	if n != 4 {
		t.Errorf("WriteFid() n = %d, want 4", n)
	}

	// Read should return the error
	buf := make([]byte, 100)
	readN, _ := ask.ReadFid(fid, buf, 0)
	response := string(buf[:readN])

	if len(response) < 6 || response[:6] != "Error:" {
		t.Errorf("ReadFid() = %q, should start with 'Error:'", response)
	}
}

func TestAskFile_SessionIsolation(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "response"

	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	fid1 := uint32(1)
	fid2 := uint32(2)

	// Write to fid1
	mock.askResponse = "response for fid1"
	ask.WriteFid(fid1, []byte("prompt1"), 0)

	// Write to fid2
	mock.askResponse = "response for fid2"
	ask.WriteFid(fid2, []byte("prompt2"), 0)

	// Read from fid1 - should get fid1's response
	buf := make([]byte, 100)
	n, _ := ask.ReadFid(fid1, buf, 0)
	response1 := string(buf[:n])
	if response1 != "response for fid1\n" {
		t.Errorf("fid1 ReadFid() = %q, want %q", response1, "response for fid1\n")
	}

	// Read from fid2 - should get fid2's response
	n, _ = ask.ReadFid(fid2, buf, 0)
	response2 := string(buf[:n])
	if response2 != "response for fid2\n" {
		t.Errorf("fid2 ReadFid() = %q, want %q", response2, "response for fid2\n")
	}
}

func TestAskFile_CloseFid(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "response"

	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	fid := uint32(1)

	// Write to create session
	ask.WriteFid(fid, []byte("test"), 0)

	// Session should exist
	session := sm.Get(fid)
	if session == nil {
		t.Fatal("session should exist after WriteFid")
	}

	// Close the fid
	ask.CloseFid(fid)

	// Session should be removed
	session = sm.Get(fid)
	if session != nil {
		t.Error("session should be removed after CloseFid")
	}
}

func TestAskFile_Stat(t *testing.T) {
	mock := NewMockBackend()
	mock.askResponse = "Hello!"

	sm := llm.NewSessionManager(mock)
	ask := NewAskFile(sm)

	// Stat without fid context returns 0 length
	stat := ask.Stat()
	if stat.Length != 0 {
		t.Errorf("Stat().Length = %d, want 0", stat.Length)
	}
}

func TestAskFile_ResponseNewline(t *testing.T) {
	mock := NewMockBackend()
	sm := llm.NewSessionManager(mock)

	fid := uint32(1)

	// Response without trailing newline
	mock.askResponse = "No newline"
	ask := NewAskFile(sm)
	ask.WriteFid(fid, []byte("test"), 0)

	buf := make([]byte, 100)
	n, _ := ask.ReadFid(fid, buf, 0)
	response := string(buf[:n])

	// Should have newline added
	if response[len(response)-1] != '\n' {
		t.Errorf("Response should end with newline, got %q", response)
	}

	// New session for second test
	fid2 := uint32(2)

	// Response with trailing newline already
	mock.askResponse = "Has newline\n"
	ask.WriteFid(fid2, []byte("test"), 0)

	n, _ = ask.ReadFid(fid2, buf, 0)
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
