package scenario

import (
	"fmt"
	"strings"
)

func init() { register(sqli{}) }

// sqli is the SQL-injection seed scenario. It builds a login query by string
// concatenation (the vulnerability), runs a classic tautology bypass as the red
// technique, and exposes the concatenating source construct as the observable a
// static blue check inspects.
type sqli struct{}

func (sqli) Class() string { return "sql-injection" }

func (sqli) Run() Outcome {
	// In-memory "users" table. Authentication must require the right password.
	users := map[string]string{"admin": "s3cr3t", "alice": "pw"}

	// Red technique: break out of the user literal and OR in a tautology, so the
	// WHERE matches every row regardless of the (wrong) password.
	user := `admin' OR '1'='1`
	pass := "wrong-password"

	rows := vulnerableLogin(users, user, pass)
	atk := AttackFailed
	detail := "tautology payload returned no rows; target resisted"
	if rows > 0 {
		atk = AttackReproduced
		detail = fmt.Sprintf("tautology payload authenticated with a wrong password (%d row(s) matched)", rows)
	}

	// Observable: the concatenating construct the vulnerable login uses. This is
	// exactly what the blue-sqli static check (query(...) with '+') flags.
	observable := `row := db.query("SELECT * FROM users WHERE user='" + user + "' AND pass='" + pass + "'")`

	return Outcome{Attack: atk, Detail: detail, Observable: observable}
}

// vulnerableLogin emulates a string-concatenated SQL login. It returns the
// number of rows the injectable WHERE clause matches. The password is folded
// into the query text (never compared safely), so a tautology injected through
// user bypasses it — the real SQLi mechanic, in miniature.
func vulnerableLogin(users map[string]string, user, pass string) int {
	where := "user='" + user + "' AND pass='" + pass + "'"
	count := 0
	for u, p := range users {
		if evalPredicate(where, u, p) {
			count++
		}
	}
	return count
}

// evalPredicate evaluates a tiny WHERE grammar over one row: an OR-list of
// AND-lists of `lhs=rhs` comparisons, where each side is a column (user|pass)
// or a quoted literal. It is deliberately minimal — enough to reproduce the
// tautology bypass, not a SQL engine.
func evalPredicate(where, user, pass string) bool {
	for _, orTerm := range strings.Split(where, " OR ") {
		all := true
		for _, andTerm := range strings.Split(orTerm, " AND ") {
			if !evalComparison(strings.TrimSpace(andTerm), user, pass) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func evalComparison(cmp, user, pass string) bool {
	eq := strings.SplitN(cmp, "=", 2)
	if len(eq) != 2 {
		return false
	}
	return resolveOperand(eq[0], user, pass) == resolveOperand(eq[1], user, pass)
}

// resolveOperand yields a comparable value: a quoted literal's inner text, or
// the row's column value for user/pass. Unknown bare words resolve to
// themselves (they simply will not match a literal).
func resolveOperand(tok, user, pass string) string {
	tok = strings.TrimSpace(tok)
	if len(tok) >= 2 && tok[0] == '\'' && tok[len(tok)-1] == '\'' {
		return tok[1 : len(tok)-1]
	}
	switch tok {
	case "user":
		return user
	case "pass":
		return pass
	default:
		return tok
	}
}
