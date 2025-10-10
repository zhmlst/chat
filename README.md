# chat — lightweight QUIC-based chat server & client for Go

A tiny, dependency-light library that provides a simple abstraction for building chat servers and clients over QUIC.
Designed for embedding into applications: session-driven API with channels for input/output and an ergonomic logging hook.

## Example

```go
inmemTokenRepo := make(map[[16]byte]struct{})

server := chat.NewServer(
    chat.ServerOptions.Handler(func(ctx context.Context, s *chat.Session) {
        in, out := s.Input(ctx), s.Output(ctx)
        defer close(out)
        out <- []byte("hello from server")
        // echo messaging
        for {
            select {
                case <-ctx.Done():
                    return
                case msg := <-in:
                    out <- msg
            }
        }
    }),
    chat.ServerOptions.TokenRepo(inmemTokenRepo),
    chat.ServerOptions.Logger(func(lvl chat.LogLevel, msg string, args ...any) {
        fmt.Println(lvl, msg, args)
    }),
)
```
## Install

```bash
go get github.com/zhmlst/chat
```
