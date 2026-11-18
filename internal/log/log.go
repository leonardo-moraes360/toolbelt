// Package log provides structured logging with Synthwave-themed colors.
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Synthwave theme ANSI colors (from assets/theme/tokens.json).
const (
	ColorReset   = "\033[0m"
	ColorPrimary = "\033[38;5;134m" // Purple #982BD4
	ColorAccent  = "\033[38;5;51m"  // Cyan #0CD7FF
	ColorError   = "\033[38;5;198m" // Pink #F9278C
	ColorSuccess = "\033[38;5;43m"  // Green #2DD4A7
	ColorWarning = "\033[38;5;214m" // Yellow #FFB020
	ColorText    = "\033[38;5;15m"  // White #FFFFFF
	ColorMuted   = "\033[38;5;249m" // Gray #A099B0
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
	useJSON       bool
	useColor      bool
)

// Init: json=false uses the colored handler (honoring NO_COLOR).
func Init(json bool) {
	InitWithWriter(os.Stderr, json)
}

func InitWithWriter(w io.Writer, json bool) {
	once.Do(func() {
		useJSON = json
		useColor = !json && shouldUseColor()

		var handler slog.Handler
		if json {
			handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
		} else {
			handler = NewSynthwaveHandler(w, useColor)
		}
		defaultLogger = slog.New(handler)
		slog.SetDefault(defaultLogger)
	})
}

func shouldUseColor() bool {
	// https://no-color.org/
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	stat, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	return true
}

func UseColor() bool {
	return useColor
}

func UseJSON() bool {
	return useJSON
}

func Logger() *slog.Logger {
	if defaultLogger == nil {
		Init(false)
	}
	return defaultLogger
}

func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

func WithContext(ctx context.Context) *slog.Logger {
	return Logger().With("trace_id", ctx.Value("trace_id"))
}

type SynthwaveHandler struct {
	w        io.Writer
	useColor bool
	level    slog.Level
	attrs    []slog.Attr
	groups   []string
}

func NewSynthwaveHandler(w io.Writer, useColor bool) *SynthwaveHandler {
	return &SynthwaveHandler{
		w:        w,
		useColor: useColor,
		level:    slog.LevelDebug,
	}
}

func (h *SynthwaveHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *SynthwaveHandler) Handle(_ context.Context, r slog.Record) error { //nolint:gocritic // slog.Handler interface requires value receiver
	var levelColor, levelText string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = ColorMuted
		levelText = "DBG"
	case slog.LevelInfo:
		levelColor = ColorAccent
		levelText = "INF"
	case slog.LevelWarn:
		levelColor = ColorWarning
		levelText = "WRN"
	case slog.LevelError:
		levelColor = ColorError
		levelText = "ERR"
	default:
		levelColor = ColorText
		levelText = "???"
	}

	var buf []byte

	if h.useColor {
		buf = append(buf, ColorMuted...)
	}
	buf = append(buf, r.Time.Format("15:04:05")...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}
	buf = append(buf, ' ')

	if h.useColor {
		buf = append(buf, levelColor...)
	}
	buf = append(buf, levelText...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}
	buf = append(buf, ' ')

	if h.useColor {
		buf = append(buf, ColorText...)
	}
	buf = append(buf, r.Message...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}

	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		if h.useColor {
			buf = append(buf, ColorPrimary...)
		}
		buf = append(buf, a.Key...)
		if h.useColor {
			buf = append(buf, ColorReset...)
		}
		buf = append(buf, '=')
		if h.useColor {
			buf = append(buf, ColorAccent...)
		}
		buf = append(buf, a.Value.String()...)
		if h.useColor {
			buf = append(buf, ColorReset...)
		}
		return true
	})

	buf = append(buf, '\n')
	_, err := h.w.Write(buf)
	return err
}

func (h *SynthwaveHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := *h
	newH.attrs = append(newH.attrs, attrs...)
	return &newH
}

func (h *SynthwaveHandler) WithGroup(name string) slog.Handler {
	newH := *h
	newH.groups = append(newH.groups, name)
	return &newH
}
