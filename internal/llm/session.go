// Package llm provides LLM backends for the 9P filesystem.
package llm

import (
	"context"
	"encoding/json"
	"sync"
)

// Session holds per-fid conversation state.
// Each fid that opens the ask file gets its own session with isolated history.
type Session struct {
	ID           uint32
	messages     []Message
	lastResponse string
	lastTokens   int
	totalTokens  int
	mu           sync.RWMutex
}

// NewSession creates a new session for the given fid.
func NewSession(fid uint32) *Session {
	return &Session{
		ID:       fid,
		messages: make([]Message, 0),
	}
}

// Messages returns a copy of the session's conversation history.
func (s *Session) Messages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// MessagesJSON returns the session's conversation history as JSON.
func (s *Session) MessagesJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.messages, "", "  ")
}

// AddMessage adds a message to the session's history.
func (s *Session) AddMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, Message{Role: role, Content: content})
}

// AddSystemMessage adds a system message to the session's history.
func (s *Session) AddSystemMessage(content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append([]Message{{Role: "system", Content: content}}, s.messages...)
}

// SetLastResponse sets the last response for this session.
func (s *Session) SetLastResponse(response string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastResponse = response
}

// LastResponse returns the last response for this session.
func (s *Session) LastResponse() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastResponse
}

// LastTokens returns the token count from the last response.
func (s *Session) LastTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastTokens
}

// TotalTokens returns cumulative token count for this session.
func (s *Session) TotalTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalTokens
}

// SetTokens updates the token counts for this session.
func (s *Session) SetTokens(last, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTokens = last
	s.totalTokens = total
}

// AddTokens adds to the token counts for this session.
func (s *Session) AddTokens(tokens int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTokens = tokens
	s.totalTokens += tokens
}

// Reset clears the session's conversation history.
func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]Message, 0)
	s.lastResponse = ""
	s.lastTokens = 0
	s.totalTokens = 0
}

// SessionManager maps fids to sessions and delegates to a shared backend.
type SessionManager struct {
	sessions map[uint32]*Session
	backend  Backend // shared backend for API calls and global settings
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager with the given backend.
func NewSessionManager(backend Backend) *SessionManager {
	return &SessionManager{
		sessions: make(map[uint32]*Session),
		backend:  backend,
	}
}

// Backend returns the underlying shared backend.
func (sm *SessionManager) Backend() Backend {
	return sm.backend
}

// GetOrCreate returns the session for the given fid, creating one if necessary.
func (sm *SessionManager) GetOrCreate(fid uint32) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s, ok := sm.sessions[fid]; ok {
		return s
	}
	s := NewSession(fid)
	sm.sessions[fid] = s
	return s
}

// Get returns the session for the given fid, or nil if it doesn't exist.
func (sm *SessionManager) Get(fid uint32) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[fid]
}

// Remove removes the session for the given fid.
func (sm *SessionManager) Remove(fid uint32) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, fid)
}

// Reset clears the session for the given fid (but keeps the session).
func (sm *SessionManager) Reset(fid uint32) {
	session := sm.GetOrCreate(fid)
	session.Reset()
}

// Ask sends a prompt using the session's conversation history.
// The response is stored in the session and returned.
func (sm *SessionManager) Ask(ctx context.Context, fid uint32, prompt string) (string, error) {
	session := sm.GetOrCreate(fid)

	// Get current history before adding new message
	history := session.Messages()

	// Use backend's AskWithHistory - it doesn't modify backend state
	response, tokens, err := sm.backend.AskWithHistory(ctx, history, prompt)
	if err != nil {
		session.SetLastResponse("Error: " + err.Error())
		return "", err
	}

	// Add user message and assistant response to session history
	session.AddMessage("user", prompt)
	session.AddMessage("assistant", response)
	session.AddTokens(tokens)
	session.SetLastResponse(response)

	return response, nil
}

// ContextLimit returns the model's context window limit from the backend.
func (sm *SessionManager) ContextLimit() int {
	return sm.backend.ContextLimit()
}
