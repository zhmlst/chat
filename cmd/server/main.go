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

type InmemTokenRepo map[[16]byte]struct{}

func (i InmemTokenRepo) SaveToken(_ context.Context, tok [16]byte) error {
	i[tok] = struct{}{}
	return nil
}
func (i InmemTokenRepo) HasToken(_ context.Context, tok [16]byte) (bool, error) {
	_, ok := i[tok]
	return ok, nil
}

func main() {
	logfile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return
	}
	lgr := slog.New(slog.NewTextHandler(io.MultiWriter(logfile, os.Stdout), &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	inmemTokenRepo := make(InmemTokenRepo)
	server := chat.NewServer(
		chat.ServerOptions.Handler(func(ctx context.Context, s *chat.Session) {
			lgr.Info("session started")
			in, out := s.Input(ctx), s.Output(ctx)
			defer func() { close(out); lgr.Info("session stopped") }()

			out <- []byte("hello from server")
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-in:
					out <- msg
				}
			}
		}),
		chat.ServerOptions.Logger(func(lvl chat.LogLevel, msg string, arg ...any) {
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
		chat.ServerOptions.TokenRepo(inmemTokenRepo),
	)

	lgr.Info("starting server")
	go func() {
		if err := server.Run(); err != nil {
			lgr.Error("server run: %v", "error", err)
			return
		}
	}()

	<-ctx.Done()
	lgr.Info("shutting down server")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := server.Shutdown(ctx); err != nil {
		lgr.Error("server shutdown: %v", "error", err)
	}
}
