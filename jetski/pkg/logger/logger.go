package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Level slog.Level

const (
	LevelDebug Level = Level(slog.LevelDebug)
	LevelInfo  Level = Level(slog.LevelInfo)
	LevelWarn  Level = Level(slog.LevelWarn)
	LevelError Level = Level(slog.LevelError)
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
)

type Logger struct {
	mu     sync.RWMutex
	logger *slog.Logger
	level  Level
	format Format
}

type Config struct {
	Level      Level
	Format     Format
	Output     io.Writer
	Structured bool
}

func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	var logHandler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.Level(cfg.Level),
	}

	switch cfg.Format {
	case FormatJSON:
		logHandler = slog.NewJSONHandler(cfg.Output, opts)
	case FormatText:
		if cfg.Structured {
			logHandler = slog.NewTextHandler(cfg.Output, opts)
		} else {
			logHandler = &simpleTextHandler{
				handler: slog.NewTextHandler(cfg.Output, opts),
				output:  cfg.Output,
			}
		}
	default:
		logHandler = slog.NewTextHandler(cfg.Output, opts)
	}

	return &Logger{
		logger: slog.New(logHandler),
		level:  cfg.Level,
		format: cfg.Format,
	}
}

func NewDefault() *Logger {
	return New(Config{
		Level:      LevelInfo,
		Format:     FormatText,
		Output:     os.Stdout,
		Structured: false,
	})
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}

func (l *Logger) With(args ...any) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Logger{
		logger: l.logger.With(args...),
		level:  l.level,
		format: l.format,
	}
}

func (l *Logger) WithGroup(name string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &Logger{
		logger: l.logger.WithGroup(name),
		level:  l.level,
		format: l.format,
	}
}

type simpleTextHandler struct {
	handler slog.Handler
	output  io.Writer
}

func (h *simpleTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *simpleTextHandler) Handle(ctx context.Context, r slog.Record) error {
	msg := r.Message
	ts := r.Time.Format("2006-01-02 15:04:05")

	var formatted string
	if r.Level >= slog.LevelError {
		formatted = fmt.Sprintf("[%s] ERROR: %s", ts, msg)
	} else if r.Level >= slog.LevelWarn {
		formatted = fmt.Sprintf("[%s] WARN: %s", ts, msg)
	} else if r.Level >= slog.LevelInfo {
		formatted = fmt.Sprintf("[%s] INFO: %s", ts, msg)
	} else {
		formatted = fmt.Sprintf("[%s] DEBUG: %s", ts, msg)
	}

	r.Attrs(func(a slog.Attr) bool {
		formatted += fmt.Sprintf(" %s=%v", a.Key, a.Value)
		return true
	})

	fmt.Fprintln(h.output, formatted)
	return nil
}

func (h *simpleTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.handler.WithAttrs(attrs)
}

func (h *simpleTextHandler) WithGroup(name string) slog.Handler {
	return h.handler.WithGroup(name)
}

func ParseLevel(level string) Level {
	switch level {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN":
		return LevelWarn
	case "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

func ParseFormat(format string) Format {
	switch format {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return FormatText
	}
}
