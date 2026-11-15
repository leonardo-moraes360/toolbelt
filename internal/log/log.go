// Package log fornece logging estruturado com suporte a cores Synthwave.
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Cores ANSI do tema Synthwave (de assets/theme/tokens.json)
const (
	ColorReset   = "\033[0m"
	ColorPrimary = "\033[38;5;134m" // Roxo #982BD4
	ColorAccent  = "\033[38;5;51m"  // Ciano #0CD7FF
	ColorError   = "\033[38;5;198m" // Rosa #F9278C
	ColorSuccess = "\033[38;5;43m"  // Verde #2DD4A7
	ColorWarning = "\033[38;5;214m" // Amarelo #FFB020
	ColorText    = "\033[38;5;15m"  // Branco #FFFFFF
	ColorMuted   = "\033[38;5;249m" // Cinza #A099B0
)

var (
	defaultLogger *slog.Logger
	once          sync.Once
	useJSON       bool
	useColor      bool
)

// Init inicializa o logger global.
// Se json=true, usa slog.JSONHandler.
// Se json=false, usa um handler com cores (respeitando NO_COLOR).
func Init(json bool) {
	InitWithWriter(os.Stderr, json)
}

// InitWithWriter inicializa o logger global com um writer específico.
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

// shouldUseColor verifica se deve usar cores no terminal.
func shouldUseColor() bool {
	// Respeita NO_COLOR (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Verifica se TERM suporta cores
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	// Verifica se stderr é um terminal
	stat, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return false // não é um terminal
	}
	return true
}

// UseColor retorna true se o logger está usando cores.
func UseColor() bool {
	return useColor
}

// UseJSON retorna true se o logger está em modo JSON.
func UseJSON() bool {
	return useJSON
}

// Logger retorna o logger global.
func Logger() *slog.Logger {
	if defaultLogger == nil {
		Init(false)
	}
	return defaultLogger
}

// Debug loga uma mensagem de debug.
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info loga uma mensagem informativa.
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn loga um aviso.
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error loga um erro.
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// With retorna um logger com atributos adicionais.
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// WithContext retorna um logger com o contexto.
func WithContext(ctx context.Context) *slog.Logger {
	return Logger().With("trace_id", ctx.Value("trace_id"))
}

// SynthwaveHandler é um handler slog com cores do tema Synthwave.
type SynthwaveHandler struct {
	w        io.Writer
	useColor bool
	level    slog.Level
	attrs    []slog.Attr
	groups   []string
}

// NewSynthwaveHandler cria um novo handler com cores Synthwave.
func NewSynthwaveHandler(w io.Writer, useColor bool) *SynthwaveHandler {
	return &SynthwaveHandler{
		w:        w,
		useColor: useColor,
		level:    slog.LevelDebug,
	}
}

// Enabled implementa slog.Handler.
func (h *SynthwaveHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle implementa slog.Handler.
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

	// Timestamp
	if h.useColor {
		buf = append(buf, ColorMuted...)
	}
	buf = append(buf, r.Time.Format("15:04:05")...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}
	buf = append(buf, ' ')

	// Level
	if h.useColor {
		buf = append(buf, levelColor...)
	}
	buf = append(buf, levelText...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}
	buf = append(buf, ' ')

	// Message
	if h.useColor {
		buf = append(buf, ColorText...)
	}
	buf = append(buf, r.Message...)
	if h.useColor {
		buf = append(buf, ColorReset...)
	}

	// Attributes
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

// WithAttrs implementa slog.Handler.
func (h *SynthwaveHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := *h
	newH.attrs = append(newH.attrs, attrs...)
	return &newH
}

// WithGroup implementa slog.Handler.
func (h *SynthwaveHandler) WithGroup(name string) slog.Handler {
	newH := *h
	newH.groups = append(newH.groups, name)
	return &newH
}
