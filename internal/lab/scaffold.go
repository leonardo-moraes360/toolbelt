package lab

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// scaffoldTmpl is a sandbox-friendly single-node lab: a target, egress denied,
// tier auto. image/build are left as placeholders the author fills for a Podman
// topology; the sandbox tier drives the in-process scenario for the class.
var scaffoldTmpl = template.Must(template.New("lab").Parse(`name: {{.Name}}
class: {{.Class}}
tier: auto
network:
  egress: deny
  internal: false
nodes:
  target:
{{- if .Target}}
    build: {{.Target}}
{{- else}}
    # in-process scenario for this class (sandbox tier); set build/image for a Podman topology
    build: builtin:{{.Class}}
{{- end}}
    expose: [8080]
limits:
  cpus: "1.0"
  memory: "512m"
  pids: 256
  timeout: "10m"
`))

// Scaffold writes labs/<name>/lab.yaml under root from the template. class ties
// the lab to a playbook pair; targetPath, when set, is recorded as the target
// build context. It refuses to overwrite an existing manifest.
func Scaffold(root, name, class, targetPath string) (string, error) {
	if !nameRE.MatchString(name) {
		return "", fmt.Errorf("lab name %q must match %s", name, nameRE)
	}
	if class == "" {
		return "", fmt.Errorf("class is required")
	}
	path := ManifestPath(root, name)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("lab %q already exists at %s", name, path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	var buf bytes.Buffer
	if err := scaffoldTmpl.Execute(&buf, struct{ Name, Class, Target string }{name, class, targetPath}); err != nil {
		return "", err
	}
	// Validate what we are about to write so a bad class/name never lands.
	if _, err := Parse(buf.Bytes()); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil { //nolint:gosec // versioned manifest, not a secret
		return "", err
	}
	return path, nil
}
