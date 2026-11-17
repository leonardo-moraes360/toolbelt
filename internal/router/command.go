package router

import (
	"fmt"
	"strings"
)

// ParseCommand quebra uma linha de comando em argv (aspas simples/duplas e
// barra invertida; sem expansão de shell).
func ParseCommand(s string) ([]string, error) {
	var (
		args    []string
		cur     strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	flush := func() {
		if inWord {
			args = append(args, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch r {
			case '"':
				quote = 0
			case '\\':
				escaped = true
			default:
				cur.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("aspas não fechadas em %q", s)
	}
	if escaped {
		return nil, fmt.Errorf("barra invertida no fim de %q", s)
	}
	flush()
	return args, nil
}
