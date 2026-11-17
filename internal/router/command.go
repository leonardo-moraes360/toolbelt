package router

import (
	"fmt"
	"strings"
)

type cmdParser struct {
	args    []string
	cur     strings.Builder
	inWord  bool
	quote   rune
	escaped bool
}

func (p *cmdParser) flush() {
	if p.inWord {
		p.args = append(p.args, p.cur.String())
		p.cur.Reset()
		p.inWord = false
	}
}

func (p *cmdParser) step(r rune) {
	switch {
	case p.escaped:
		p.cur.WriteRune(r)
		p.escaped = false
	case p.quote == '\'':
		if r == '\'' {
			p.quote = 0
		} else {
			p.cur.WriteRune(r)
		}
	case p.quote == '"':
		p.stepDouble(r)
	case r == '\\':
		p.escaped, p.inWord = true, true
	case r == '\'' || r == '"':
		p.quote, p.inWord = r, true
	case r == ' ' || r == '\t' || r == '\n':
		p.flush()
	default:
		p.cur.WriteRune(r)
		p.inWord = true
	}
}

func (p *cmdParser) stepDouble(r rune) {
	switch r {
	case '"':
		p.quote = 0
	case '\\':
		p.escaped = true
	default:
		p.cur.WriteRune(r)
	}
}

// ParseCommand quebra uma linha de comando em argv (aspas simples/duplas e
// barra invertida; sem expansão de shell).
func ParseCommand(s string) ([]string, error) {
	var p cmdParser
	for _, r := range s {
		p.step(r)
	}
	if p.quote != 0 {
		return nil, fmt.Errorf("aspas não fechadas em %q", s)
	}
	if p.escaped {
		return nil, fmt.Errorf("barra invertida no fim de %q", s)
	}
	p.flush()
	return p.args, nil
}
