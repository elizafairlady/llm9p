// llm9p exposes an LLM (Claude) as a 9P filesystem.
//
// Usage:
//
//	ANTHROPIC_API_KEY=sk-... llm9p -addr :5640
//
// Mount with:
//
//	9pfuse localhost:5640 /mnt/llm
//
// Interact:
//
//	echo "What is 2+2?" > /mnt/llm/ask
//	cat /mnt/llm/ask
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/NERVsystems/llm9p/internal/llm"
	"github.com/NERVsystems/llm9p/internal/llmfs"
	"github.com/NERVsystems/llm9p/internal/protocol"
)

func main() {
	addr := flag.String("addr", ":5640", "Address to listen on")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Get API key from environment
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create LLM client
	client := llm.NewClient(apiKey)

	// Create filesystem
	root := llmfs.NewRoot(client)

	// Create 9P server
	server := protocol.NewServer(root)
	server.SetDebug(*debug)

	// Listen
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", *addr, err)
	}

	log.Printf("llm9p listening on %s", *addr)
	log.Printf("Mount with: 9pfuse %s /mnt/llm", listener.Addr())

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		cancel()
		listener.Close()
	}()

	// Serve
	if err := server.Serve(ctx, listener); err != nil && ctx.Err() == nil {
		log.Fatalf("Server error: %v", err)
	}
}
