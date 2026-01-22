# llm9p

An LLM (Claude) exposed as a 9P filesystem.

llm9p enables users, scripts, and AI agents to interact with an LLM through standard filesystem operations. Write a prompt to a file, read the response from the same file.

## Installation

```bash
go install github.com/NERVsystems/llm9p/cmd/llm9p@latest
```

Or build from source:

```bash
git clone https://github.com/NERVsystems/llm9p
cd llm9p
go build -o llm9p ./cmd/llm9p
```

## Usage

### Start the Server

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./llm9p -addr :5640
```

### Mount the Filesystem

Using 9pfuse (Plan 9 from User Space):

```bash
mkdir -p /mnt/llm
9pfuse localhost:5640 /mnt/llm
```

On macOS with plan9port:

```bash
9 mount localhost:5640 /mnt/llm
```

### Interact with the LLM

```bash
# Ask a question
echo "What is 2+2?" > /mnt/llm/ask
cat /mnt/llm/ask

# View token usage
cat /mnt/llm/tokens

# Change model
echo "claude-3-haiku-20240307" > /mnt/llm/model

# Adjust temperature
echo "0.5" > /mnt/llm/temperature

# View conversation history
cat /mnt/llm/context

# Add a system message
echo "You are a helpful coding assistant." > /mnt/llm/context

# Reset conversation
echo "" > /mnt/llm/new

# View help
cat /mnt/llm/_example
```

## Filesystem Schema

```
/llm/
├── ask              # Write prompt, read response (same file)
├── model            # Read/write: current model name
├── temperature      # Read/write: temperature float (0.0-2.0)
├── tokens           # Read-only: last response token count
├── new              # Write anything to start fresh conversation
├── context          # Read: conversation history; Write: add system message
├── _example         # Read-only: usage examples
└── stream/          # Streaming interface
    └── chunk        # Read blocks until next chunk, EOF on completion
```

### File Behaviors

| File | Read | Write |
|------|------|-------|
| `ask` | Returns last LLM response | Sends prompt to LLM, stores response |
| `model` | Returns current model name | Sets model for subsequent requests |
| `temperature` | Returns current temperature | Sets temperature (0.0-2.0) |
| `tokens` | Returns last response token count | Permission denied |
| `new` | Permission denied | Any write resets conversation state |
| `context` | Returns JSON conversation history | Appends system message to context |
| `_example` | Returns usage examples | Permission denied |
| `stream/chunk` | Blocks until next chunk, returns it | Permission denied |

## Shell Scripting

```bash
#!/bin/sh
# ask.sh - Simple LLM query script

if [ -z "$1" ]; then
    echo "Usage: $0 <question>"
    exit 1
fi

echo "$1" > /mnt/llm/ask
cat /mnt/llm/ask
```

## Configuration

### Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:5640` | Address to listen on |
| `-debug` | `false` | Enable debug logging |

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | Yes | Your Anthropic API key |

## Default Settings

- **Model**: `claude-sonnet-4-20250514`
- **Temperature**: `0.7`
- **Max Tokens**: `4096`

## Requirements

- Go 1.21+
- Anthropic API key
- 9P client (9pfuse, plan9port, or native Plan 9)

## License

MIT
