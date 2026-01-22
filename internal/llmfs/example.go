package llmfs

import (
	"github.com/NERVsystems/llm9p/internal/protocol"
)

const exampleContent = `LLM 9P Filesystem Usage Examples
=================================

Basic Interaction:
  echo "What is 2+2?" > ask     # Send prompt to LLM
  cat ask                        # Read response

Configuration:
  cat model                      # View current model
  echo "claude-3-haiku-20240307" > model   # Change model
  cat temperature                # View current temperature (0.0-2.0)
  echo "0.5" > temperature       # Set temperature

Conversation Management:
  cat context                    # View conversation history (JSON)
  echo "You are a helpful assistant." > context  # Add system message
  echo "" > new                  # Reset conversation

Token Usage:
  cat tokens                     # View tokens from last response

Streaming (Advanced):
  echo "Tell me a story" > ask   # Start generating
  cat stream/chunk               # Read chunks as they arrive (blocks)

Shell Scripting:
  #!/bin/sh
  # Ask the LLM and get response
  echo "$1" > /mnt/llm/ask
  cat /mnt/llm/ask

Mounting (Linux/macOS):
  # Using 9pfuse (Plan 9 from User Space)
  9pfuse localhost:5640 /mnt/llm

  # Using mount_9p (macOS with plan9port)
  mount_9p localhost:5640 /mnt/llm

Environment:
  ANTHROPIC_API_KEY must be set when starting the server

Files:
  ask          Read/write: prompt goes in, response comes out
  model        Read/write: current model name
  temperature  Read/write: sampling temperature (0.0-2.0)
  tokens       Read-only: token count from last response
  new          Write-only: any write resets conversation
  context      Read: JSON history; Write: add system message
  _example     Read-only: this help text
  stream/chunk Read-only: streaming chunks (blocking)
`

// NewExampleFile creates the _example file with usage examples
func NewExampleFile() *protocol.StaticFile {
	return protocol.NewStaticFile("_example", []byte(exampleContent))
}
