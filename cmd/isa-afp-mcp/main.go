package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/afpclient"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/cache"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/repository"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/searchindex"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/theoryarchive"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driving/mcp"
	"github.com/isa-afp/isa-afp-mcp/internal/application/service"
)

func main() {
	var (
		cacheDir     string
		forceSync    bool
		offline      bool
		syncInterval time.Duration
		httpAddr     string
		logLevelStr  string
	)

	flag.StringVar(&cacheDir, "cache-dir", os.Getenv("ISA_AFP_CACHE_DIR"), "Local cache directory (default: ~/.cache/isa-afp-mcp)")
	flag.BoolVar(&forceSync, "force-sync", false, "Force sync from upstream isa-afp.org on startup")
	flag.BoolVar(&offline, "offline", false, "Offline mode: do not contact upstream network")
	flag.DurationVar(&syncInterval, "sync-interval", 24*time.Hour, "Background catalog sync interval (0 to disable)")
	flag.StringVar(&httpAddr, "http", "", "Serve HTTP SSE on address (e.g. ':8080') instead of stdio")
	flag.StringVar(&logLevelStr, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Logger MUST output to os.Stderr because stdout is reserved for MCP JSON-RPC protocol!
	var level slog.Level
	switch logLevelStr {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// 1. Initialize driven adapters (Secondary Ports)
	diskCache, err := cache.NewDiskCache(cacheDir, 24*time.Hour)
	if err != nil {
		logger.Error("Failed to initialize disk cache", "err", err)
		os.Exit(1)
	}

	repo := repository.NewMemoryCatalogRepository()
	idx := searchindex.NewBM25Index()
	extractor := theoryarchive.New()
	client := afpclient.New()

	// 2. Initialize application services (Hexagonal Core)
	theoryService := service.NewTheoryService(client, diskCache, extractor, logger)
	catalogService := service.NewCatalogService(repo, client, idx, diskCache, theoryService, logger)
	searchService := service.NewSearchService(idx)

	// 3. Warm-up catalog (from disk cache or upstream)
	initCtx, initCancel := context.WithTimeout(ctx, 45*time.Second)
	defer initCancel()

	if offline {
		logger.Info("Starting in offline mode, loading from local cache...")
		if err := catalogService.Init(initCtx, false); err != nil {
			logger.Error("Offline init failed (no cache available)", "err", err)
			os.Exit(1)
		}
	} else {
		if err := catalogService.Init(initCtx, forceSync); err != nil {
			logger.Warn("Initial catalog sync encountered an error; will retry in background", "err", err)
		}
	}

	// 4. Background periodic catalog sync
	if !offline && syncInterval > 0 {
		go func() {
			ticker := time.NewTicker(syncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					logger.Info("Executing scheduled catalog background sync...")
					syncCtx, syncCancel := context.WithTimeout(context.Background(), 2*time.Minute)
					if err := catalogService.Sync(syncCtx); err != nil {
						logger.Warn("Scheduled catalog sync failed", "err", err)
					}
					syncCancel()
				}
			}
		}()
	}

	// 5. Initialize driving MCP server adapter (Primary Port)
	srv := mcp.NewServer(
		searchService,
		catalogService,
		catalogService,
		catalogService,
		theoryService,
		catalogService,
		logger,
	)

	// 6. Serve MCP
	if httpAddr != "" {
		if err := srv.ServeSSE(httpAddr); err != nil {
			logger.Error("HTTP SSE server failed", "err", err)
			os.Exit(1)
		}
	} else {
		if err := srv.ServeStdio(); err != nil {
			logger.Error("Stdio server failed", "err", err)
			os.Exit(1)
		}
	}
}
