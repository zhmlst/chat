package chat

type serverConfig struct {
	address string
}

var defaultServerConfig = serverConfig{
	address: "localhost:4242",
}

// Server provides chat sessions.
type Server struct{ cfg serverConfig }

// ServerOption applies option to server.
type ServerOption func(cfg *serverConfig)

// ServerOptions provides available options for server.
var ServerOptions serverOptionsNamespace

type serverOptionsNamespace struct{}

func (serverOptionsNamespace) Address(addr string) ServerOption {
	return func(cfg *serverConfig) {
		cfg.address = addr
	}
}

// NewServer creates a server with specified options.
func NewServer(opts ...ServerOption) *Server {
	cfg := defaultServerConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Server{cfg: cfg}
}
