package router

import (
	"fmt"
	"strings"
)

// cmdLexer é o estado do tokenizador de ParseCommand.
type cmdLexer struct {
	args    []string
	cur     strings.Builder
	inWord  bool
	quote   rune
	escaped bool
}

func (l *cmdLexer) flush() {
	if l.inWord {
		l.args = append(l.args, l.cur.String())
		l.cur.Reset()
		l.inWord = false
	}
}

// inSingleQuote trata um caractere dentro de aspas simples (tudo literal).
func (l *cmdLexer) inSingleQuote(r rune) {
	if r == '\'' {
		l.quote = 0
	} else {
		l.cur.WriteRune(r)
	}
}

// inDoubleQuote trata um caractere dentro de aspas duplas (só \ escapa).
func (l *cmdLexer) inDoubleQuote(r rune) {
	switch r {
	case '"':
		l.quote = 0
	case '\\':
		l.escaped = true
	default:
		l.cur.WriteRune(r)
	}
}

// bare trata um caractere fora de aspas e de escape.
func (l *cmdLexer) bare(r rune) {
	switch {
	case r == '\\':
		l.escaped, l.inWord = true, true
	case r == '\'' || r == '"':
		l.quote, l.inWord = r, true
	case r == ' ' || r == '\t' || r == '\n':
		l.flush()
	default:
		l.cur.WriteRune(r)
		l.inWord = true
	}
}

func (l *cmdLexer) step(r rune) {
	switch {
	case l.escaped:
		l.cur.WriteRune(r)
		l.escaped = false
	case l.quote == '\'':
		l.inSingleQuote(r)
	case l.quote == '"':
		l.inDoubleQuote(r)
	default:
		l.bare(r)
	}
}

// ParseCommand quebra uma linha de comando em argv (aspas simples/duplas e
// barra invertida; sem expansão de shell).
func ParseCommand(s string) ([]string, error) {
	var l cmdLexer
	for _, r := range s {
		l.step(r)
	}
	if l.quote != 0 {
		return nil, fmt.Errorf("aspas não fechadas em %q", s)
	}
	if l.escaped {
		return nil, fmt.Errorf("barra invertida no fim de %q", s)
	}
	l.flush()
	return l.args, nil
}
