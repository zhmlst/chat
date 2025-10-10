package chat

// LogLevel represents the severity level of a log message.
//
//go:generate enumer -output=loglevel.go -text -transform=upper -trimprefix=LogLevel -type=LogLevel
type LogLevel int8

const (
	// LogLevelDebug represents debug-level log messages.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo represents informational log messages.
	LogLevelInfo
	// LogLevelWarn represents warning-level log messages.
	LogLevelWarn
	// LogLevelError represents error-level log messages.
	LogLevelError
)

// Logger defines a logging function.
type Logger func(lvl LogLevel, msg string, arg ...any)

// Debug logs a message at debug level.
func (l Logger) Debug(msg string) {
	l(LogLevelDebug, msg)
}

// Info logs a message at info level.
func (l Logger) Info(msg string) {
	l(LogLevelInfo, msg)
}

// Warn logs a message at warning level.
func (l Logger) Warn(msg string) {
	l(LogLevelWarn, msg)
}

// Error logs a message at error level.
func (l Logger) Error(msg string) {
	l(LogLevelError, msg)
}

// With returns a new logger that appends additional arguments to every log call.
func (l Logger) With(arg ...any) Logger {
	return func(lvl LogLevel, msg string, a ...any) {
		l(lvl, msg, append(arg, a...)...)
	}
}

// NopLogger is a no-operation logger that discards all log messages.
func NopLogger(LogLevel, string, ...any) {}
