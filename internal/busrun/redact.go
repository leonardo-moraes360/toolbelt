package busrun

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/execenv"
	"github.com/leonardo-moraes360/toolbelt/internal/mem"
)

const redacted = "[REDACTED]"

// minSecretLen: valores (e palavras de valores) mais curtos que isto, vindos
// do ambiente ou de secret_files, não são redigidos (evita apagar palavras
// comuns). pass_env usa minPassEnvLen: ali o humano declarou o segredo.
const (
	minSecretLen  = 6
	minPassEnvLen = 4
)

// Palavras de valor que são esquema/cabeçalho, não segredo.
var notSecretWords = map[string]bool{
	"authorization": true, "authorization:": true, "bearer": true, "basic": true, "token": true,
}

type secretPattern struct {
	re   *regexp.Regexp
	repl string
}

// O harness não sabe onde o filho guarda segredos: além dos valores
// conhecidos, a saída do filho passa por padrões genéricos antes do audit.
var genericSecretPatterns = []secretPattern{
	// cookies: o valor inteiro até o fim da linha
	{regexp.MustCompile(`(?im)((?:^|[\s"'])(?:set-)?cookie"?[ \t]*:[ \t]*)[^\r\n]+`), "${1}" + redacted},
	// cabeçalho de credencial em forma de cabeçalho (início de linha, aspas
	// ou -H): esquema opcional + valor
	{regexp.MustCompile(`(?im)((?:^|[\s"'>]|-H[ \t]+["']?)[ \t]*(?:proxy-)?(?:authorization|x-api-key|api[-_]?key)"?[ \t]*:[ \t]*"?)(?:(?:bearer|basic|token)[ \t]+)?[^\s"',;]+`), "${1}" + redacted},
	// JWT (segmentos separados por ponto escapam da regra de token longo)
	{regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]*`), redacted},
	// nome de segredo seguido de = ou : (inclusive chave JSON), esquema opcional
	// (SECRET_KEY, AWS_ACCESS_KEY_ID, "api_key":"x"); valor só de dígitos
	// fica (contadores como max_tokens=4096)
	{regexp.MustCompile(`(?i)\b((?:[A-Za-z0-9_\-]*(?:token|secret|passw(?:or)?d|pwd|credential)|[A-Za-z0-9_\-]*[_\-]key|key)[A-Za-z0-9_\-]*"?[ \t]*[:=][ \t]*)(?:(?:bearer|basic|token)[ \t]+)?("[^"]*"|'[^']*'|[^\s,;&"'}]*[^\d\s,;&"'}][^\s,;&"'}]*)`), "${1}" + redacted},
	// credencial em URL: userinfo e query string
	{regexp.MustCompile(`(://)[^/\s:@]+:[^/\s]*@`), "${1}" + redacted + "@"}, // até o último @ (senha com @)
	{regexp.MustCompile(`(?i)([?&](?:token|key|sig|signature|secret|access_token|auth|code)=)[^&\s"']+`), "${1}" + redacted},
}

// Sequências longas com letras e dígitos parecem tokens (inclusive base64
// com `/`). Ficam: UUIDs (ids de sessão/run) e caminhos absolutos.
var (
	longTokenRe = regexp.MustCompile(`[A-Za-z0-9_\-+/=]{32,}`)
	uuidRe      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	hasLetterRe = regexp.MustCompile(`[A-Za-z]`)
	hasDigitRe  = regexp.MustCompile(`\d`)
)

// redactor redige texto que vai ao audit (saída do filho, erros). Os valores
// são lidos no Open: segredo trocado exige reiniciar o harness.
type redactor struct {
	known *strings.Replacer
}

// newRedactor junta os valores conhecidos: pass_env, variáveis de segredo do
// processo do harness e valores dos secret_files da config.
func newRedactor(cfg *Config) (*redactor, error) {
	set := map[string]bool{}
	add := func(v string, minLen int) {
		for _, c := range secretCandidates(v, minLen) {
			set[c] = true
		}
	}
	for _, name := range cfg.PassEnv {
		add(os.Getenv(strings.TrimSpace(name)), minPassEnvLen)
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && execenv.IsSecret(k) {
			add(v, minSecretLen)
		}
	}
	for _, p := range cfg.SecretFiles {
		vals, err := readSecretFile(p)
		if err != nil {
			return nil, err
		}
		for _, v := range vals {
			add(v, minSecretLen)
		}
	}
	vals := make([]string, 0, len(set))
	for v := range set {
		vals = append(vals, v)
	}
	// Replacer compara na ordem dos argumentos: mais longos primeiro.
	sort.Slice(vals, func(i, j int) bool {
		if len(vals[i]) != len(vals[j]) {
			return len(vals[i]) > len(vals[j])
		}
		return vals[i] < vals[j]
	})
	pairs := make([]string, 0, 2*len(vals))
	for _, v := range vals {
		pairs = append(pairs, v, redacted)
	}
	return &redactor{known: strings.NewReplacer(pairs...)}, nil
}

// secretCandidates devolve o valor inteiro, cada sufixo após espaço
// ("Authorization: Bearer X" também redige "Bearer X") e cada palavra como o
// sh a separaria (espaço/tab/quebra de linha, sem aspas): um env carregado
// como shell imprime palavras soltas do valor em erros.
func secretCandidates(v string, minLen int) []string {
	v = strings.TrimSpace(v)
	var out []string
	if len(v) >= minLen {
		out = append(out, v) // o valor inteiro sempre, mesmo que seja "token"
	}
	keep := func(c string) {
		if len(c) >= minLen && !notSecretWords[strings.ToLower(c)] {
			out = append(out, c)
		}
	}
	fields := strings.Fields(v)
	for i := range fields {
		if i > 0 {
			keep(strings.Join(fields[i:], " "))
		}
		keep(strings.Trim(fields[i], `"'`))
	}
	return out
}

// readSecretFile lê KEY=VALOR (comentários, linhas vazias, `export`, aspas e
// comentário no fim da linha tolerados) sem executar nada. Exige arquivo
// regular com permissão 0600 ou mais fechada, como o arquivo de wake.
// Ausente ou ilegível é erro (fail-closed: sem os valores, o audit vazaria).
func readSecretFile(path string) ([]string, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("secret_files: caminho %q precisa ser absoluto", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("secret_files: %w", err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("secret_files: %s não é arquivo regular", path)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("secret_files: %s com permissão %o: use chmod 600", path, st.Mode().Perm())
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("secret_files: %w", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		_, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, envValue(strings.TrimSpace(v)))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("secret_files %s: %w", path, err)
	}
	return out, nil
}

// envValue tira aspas externas ou, sem aspas, o comentário " #..." final.
func envValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	if i := strings.Index(v, "\t#"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// redact aplica, nesta ordem: valores conhecidos, padrões do mem, padrões
// genéricos e, por fim, mascara sequências com cara de token.
func (r *redactor) redact(s string) string {
	if s == "" {
		return s
	}
	if r != nil && r.known != nil {
		s = r.known.Replace(s)
	}
	s = mem.RedactText(s)
	for _, p := range genericSecretPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return longTokenRe.ReplaceAllStringFunc(s, func(m string) string {
		if uuidRe.MatchString(m) || strings.HasPrefix(m, "/") || !hasLetterRe.MatchString(m) || !hasDigitRe.MatchString(m) {
			return m
		}
		return redacted
	})
}
