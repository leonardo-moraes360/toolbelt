package merge

import (
	"fmt"
	"strings"
)

type Strategy string

const (
	StrategyFail     Strategy = "fail"
	StrategyLastWins Strategy = "last_wins"
	StrategySemantic Strategy = "semantic"
	StrategyPrompt   Strategy = "prompt"
)

const strategyChoices = "fail, last_wins, semantic or prompt"

// ParseStrategy treats "" as fail.
func ParseStrategy(s string) (Strategy, error) {
	switch v := strings.ToLower(strings.TrimSpace(s)); v {
	case "", "fail":
		return StrategyFail, nil
	case "last_wins", "last-wins":
		return StrategyLastWins, nil
	case "semantic":
		return StrategySemantic, nil
	case "prompt":
		return StrategyPrompt, nil
	default:
		return "", fmt.Errorf("invalid strategy %q (use %s)", s, strategyChoices)
	}
}

// resolving reports strategies that resolve conflicts hunk by hunk through
// the semantic resolver (prompt runs it first).
func (s Strategy) resolving() bool { return s == StrategySemantic || s == StrategyPrompt }
