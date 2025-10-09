package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zhmlst/chat"
)

func main() {
	lgr := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	client := chat.NewClient(
		chat.ClientOptions.Servers([]string{"localhost:4242"}),
	)
	if err := client.Dial(ctx); err != nil {
		lgr.Error("failed while dial", "error", err)
		cancel()
	}
}
