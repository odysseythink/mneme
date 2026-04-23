package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/config"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/mcp"
	"github.com/ranwei/claude-context/pkg/splitter"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

func main() {
	cfg := config.FromEnv()

	if cfg.EmbeddingAPIKey == "" {
		log.Fatal("EMBEDDING_API_KEY environment variable is required")
	}

	store := vectordb.NewStore()
	if err := store.Initialize(cfg.DBPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	var provider pkg.EmbeddingProvider
	switch cfg.EmbeddingProvider {
	case "siliconflow":
		provider = embedding.NewSiliconFlowProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	case "qwen":
		provider = embedding.NewQwenProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	default:
		log.Fatalf("Unknown embedding provider: %s", cfg.EmbeddingProvider)
	}

	embeddingClient := embedding.NewCachedClient(provider)

	codeSplitter := splitter.NewSplitter()

	indexer := ctxpkg.NewIndexer(store, embeddingClient, codeSplitter)
	searcher := ctxpkg.NewSearcher(store, embeddingClient)

	server := mcp.NewMCPServer(indexer, searcher)

	fmt.Fprintf(os.Stderr, "Claude Context MCP Server started\n")
	fmt.Fprintf(os.Stderr, "Embedding Provider: %s\n", cfg.EmbeddingProvider)
	fmt.Fprintf(os.Stderr, "Database: %s\n", cfg.DBPath)

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
