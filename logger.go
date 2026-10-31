package jev

import (
	"context"
	"log/slog"
)

type Logger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	With(args ...any) Logger
}

type slogAdapter struct {
	l *slog.Logger
}

func simpleSlogAdapter() *slogAdapter {
	return &slogAdapter{l: slog.Default()}
}

func (s *slogAdapter) Debug(ctx context.Context, msg string, args ...any) {
	s.l.DebugContext(ctx, msg, args...)
}

func (s *slogAdapter) Info(ctx context.Context, msg string, args ...any) {
	s.l.InfoContext(ctx, msg, args...)
}

func (s *slogAdapter) Warn(ctx context.Context, msg string, args ...any) {
	s.l.WarnContext(ctx, msg, args...)
}

func (s *slogAdapter) With(args ...any) Logger {
	return &slogAdapter{l: s.l.With(args...)}
}
