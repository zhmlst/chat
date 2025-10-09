package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhmlst/chat"
)

func main() {
	logfile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return
	}
	log := slog.New(slog.NewJSONHandler(io.MultiWriter(logfile, os.Stdout), &slog.HandlerOptions{}))

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	server := chat.NewServer(
		chat.ServerOptions.Handler(func(ctx context.Context, s *chat.Session) {
			log.Info("session started")
		}),
	)

	log.Info("starting server")
	go func() {
		if err := server.Run(); err != nil {
			log.Error("server run: %v", "error", err)
			return
		}
	}()

	<-ctx.Done()
	log.Info("shutting down server")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown: %v", "error", err)
	}
}
