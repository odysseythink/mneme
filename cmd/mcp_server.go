package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/odysseythink/mlog"
	"github.com/ranwei/mneme/pkg"
	"github.com/ranwei/mneme/pkg/config"
	ctxpkg "github.com/ranwei/mneme/pkg/context"
	"github.com/ranwei/mneme/pkg/embedding"
	"github.com/ranwei/mneme/pkg/mcp"
	"github.com/ranwei/mneme/pkg/splitter"
	"github.com/ranwei/mneme/pkg/vectordb"
)

func initLogger(logLevel string) {
	flag.Set("logtostderr", "true")
	if logLevel == "debug" {
		flag.Set("v", "2")
	}
}

func runMCPServer() {
	cfg := config.FromEnv()
	initLogger(cfg.LogLevel)

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Println("Mneme MCP Server")
		fmt.Println("Usage: Set EMBEDDING_API_KEY and run via Claude Code MCP")
		fmt.Printf("Provider: %s, Model: %s\n", cfg.EmbeddingProvider, cfg.EmbeddingModel)
		os.Exit(0)
	}

	var (
		indexer        pkg.Indexer
		searcher       pkg.Searcher
		embeddingClient *embedding.CachedClient
	)

	if cfg.EmbeddingAPIKey != "" {
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

		embeddingClient = embedding.NewCachedClient(provider)
		codeSplitter := splitter.NewSplitter()
		indexer = ctxpkg.NewIndexer(store, embeddingClient, codeSplitter, cfg.EmbeddingModel)
		searcher = ctxpkg.NewSearcher(store, embeddingClient)

		keyHint := ""
		if len(cfg.EmbeddingAPIKey) > 10 {
			keyHint = cfg.EmbeddingAPIKey[:10] + "..."
		}
		configSrc := "defaults"
		if cfg.ConfigSource != "" {
			configSrc = cfg.ConfigSource
		}
		mlog.Infof("Mneme MCP Server started: provider=%s model=%s backend=%s key=%s config=%s",
			cfg.EmbeddingProvider, cfg.EmbeddingModel, cfg.DBBackend, keyHint, configSrc)
	} else {
		mlog.Infof("Mneme MCP Server started: EMBEDDING_API_KEY not set — embedding tools unavailable; local-state tools active")
	}

	server := mcp.NewMCPServer(indexer, searcher, embeddingClient)
	if err := server.Start(); err != nil {
		mlog.Fatalf("Server error: %v", err)
	}
}
