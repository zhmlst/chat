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
	lgr := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()
	client := chat.NewClient(
		chat.ClientOptions.Servers([]string{"localhost:4242"}),
		chat.ClientOptions.Logger(func(lvl chat.LogLevel, msg string, arg ...any) {
			switch lvl {
			case chat.LogLevelDebug:
				lgr.Debug(msg, arg...)
			case chat.LogLevelInfo:
				lgr.Info(msg, arg...)
			case chat.LogLevelWarn:
				lgr.Warn(msg, arg...)
			case chat.LogLevelError:
				lgr.Error(msg, arg...)
			}
		}),
	)
	if err := client.Dial(ctx); err != nil {
		lgr.Error("failed while dial", "error", err)
		cancel()
	}
}
