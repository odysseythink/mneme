package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/odysseythink/mlog"
	"github.com/ranwei/claude-context/pkg"
	"github.com/ranwei/claude-context/pkg/config"
	ctxpkg "github.com/ranwei/claude-context/pkg/context"
	"github.com/ranwei/claude-context/pkg/embedding"
	"github.com/ranwei/claude-context/pkg/mcp"
	"github.com/ranwei/claude-context/pkg/splitter"
	"github.com/ranwei/claude-context/pkg/vectordb"
)

// initLogger configures mlog to write to stderr and sets verbosity from LOG_LEVEL.
// Stderr is used because stdout is reserved for MCP JSON-RPC communication.
func initLogger(logLevel string) {
	flag.Set("logtostderr", "true")
	if logLevel == "debug" {
		flag.Set("v", "2")
	}
}

func main() {
	cfg := config.FromEnv()
	initLogger(cfg.LogLevel)

	// Check if running in MCP health check mode (no stdin input expected)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		// Running in terminal (not piped), print info and exit
		fmt.Println("Claude Context MCP Server")
		fmt.Println("Usage: Set EMBEDDING_API_KEY and run via Claude Code MCP")
		fmt.Printf("Provider: %s, Model: %s\n", cfg.EmbeddingProvider, cfg.EmbeddingModel)
		os.Exit(0)
	}

	if cfg.EmbeddingAPIKey == "" {
		mlog.Fatal("EMBEDDING_API_KEY environment variable is required")
	}

	store, err := vectordb.NewStoreFromConfig(cfg)
	if err != nil {
		mlog.Fatalf("Invalid DB_BACKEND: %v", err)
	}
	if err := store.Initialize(cfg.DBPath); err != nil {
		mlog.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	var provider pkg.EmbeddingProvider
	switch cfg.EmbeddingProvider {
	case "siliconflow":
		provider = embedding.NewSiliconFlowProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	case "qwen":
		provider = embedding.NewQwenProvider(cfg.EmbeddingAPIKey, cfg.EmbeddingModel)
	default:
		mlog.Fatalf("Unknown embedding provider: %s", cfg.EmbeddingProvider)
	}

	embeddingClient := embedding.NewCachedClient(provider)
	codeSplitter := splitter.NewSplitter()
	indexer := ctxpkg.NewIndexer(store, embeddingClient, codeSplitter, cfg.EmbeddingModel)
	searcher := ctxpkg.NewSearcher(store, embeddingClient)
	server := mcp.NewMCPServer(indexer, searcher, embeddingClient)

	keyHint := ""
	if len(cfg.EmbeddingAPIKey) > 10 {
		keyHint = cfg.EmbeddingAPIKey[:10] + "..."
	}
	mlog.Infof("Claude Context MCP Server started: provider=%s model=%s backend=%s key=%s",
		cfg.EmbeddingProvider, cfg.EmbeddingModel, cfg.DBBackend, keyHint)

	if err := server.Start(); err != nil {
		mlog.Fatalf("Server error: %v", err)
	}
}
