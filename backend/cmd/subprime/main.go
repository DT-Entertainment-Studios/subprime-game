// Command subprime runs the Subprime game server: an authoritative, in-memory
// multiplayer board game server with an HTTP API and a WebSocket channel
// (see Spec.md §6).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/api"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/auth"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/config"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/presence"
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/room"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "subprime:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadEnv()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)

	catalog, err := board.New()
	if err != nil {
		return fmt.Errorf("invalid board catalog: %w", err)
	}

	store := auth.NewStore()
	rooms := room.NewManager(catalog)
	hub := presence.NewHub(rooms)
	server := api.New(cfg, store, rooms, hub, logger)

	done := make(chan struct{})
	go rooms.Run(done)
	go hub.Run(done)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("subprime listening", "port", cfg.Port, "staticDir", cfg.StaticDir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		close(done)
		return err
	case sig := <-signals:
		logger.Info("shutting down", "signal", sig.String())
	}

	close(done)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

func logLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
