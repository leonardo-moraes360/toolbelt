package graph

import (
	"regexp"
	"strings"
)

var (
	reJSImportNamed = regexp.MustCompile(`(?m)(?:import|export)\s+(?:type\s+)?\{([^}]+)\}\s+from\s+['"]([^'"]+)['"]`)
	reJSImportNS    = regexp.MustCompile(`(?m)import\s+\*\s+as\s+([A-Za-z_$][\w$]*)\s+from\s+['"]([^'"]+)['"]`)
	reJSImportDef   = regexp.MustCompile(`(?m)import\s+([A-Za-z_$][\w$]*)\s*(?:,\s*(?:\{[^}]+\}|\*\s+as\s+[A-Za-z_$][\w$]*))?\s+from\s+['"]([^'"]+)['"]`)
)

type jsBinding struct {
	local      string
	exportName string
	module     string
	namespace  bool
}

func extractJSBindings(root, fromRel, raw string, cfg *jsPathConfig) []jsBinding {
	var out []jsBinding
	seen := map[string]bool{}
	add := func(b jsBinding) {
		key := b.local + "\x00" + b.module + "\x00" + b.exportName
		if b.local == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, b)
	}
	moduleOf := func(imp string) string {
		if rel, ok := resolveJSImport(root, fromRel, imp, cfg); ok {
			return rel
		}
		return imp
	}
	for _, m := range reJSImportNamed.FindAllStringSubmatch(raw, -1) {
		if len(m) < 3 {
			continue
		}
		mod := moduleOf(m[2])
		for _, part := range strings.Split(m[1], ",") {
			local, exported, ok := parseJSImportSpec(part)
			if !ok {
				continue
			}
			add(jsBinding{local: local, exportName: exported, module: mod})
		}
	}
	for _, m := range reJSImportNS.FindAllStringSubmatch(raw, -1) {
		if len(m) < 3 {
			continue
		}
		add(jsBinding{local: m[1], module: moduleOf(m[2]), namespace: true})
	}
	for _, m := range reJSImportDef.FindAllStringSubmatch(raw, -1) {
		if len(m) < 3 {
			continue
		}
		name := m[1]
		if name == "type" || name == "typeof" {
			continue
		}
		add(jsBinding{local: name, exportName: "default", module: moduleOf(m[2])})
	}
	return out
}

func parseJSImportSpec(part string) (local, exported string, ok bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", "", false
	}
	part = strings.TrimPrefix(part, "type ")
	part = strings.TrimSpace(part)
	if part == "" {
		return "", "", false
	}
	if strings.Contains(part, " as ") {
		bits := strings.Split(part, " as ")
		if len(bits) != 2 {
			return "", "", false
		}
		exported = strings.TrimSpace(bits[0])
		local = strings.TrimSpace(bits[1])
		return local, exported, local != "" && exported != ""
	}
	return part, part, true
}

func resolveJSBoundCallee(raw string, bindings []jsBinding) (exportName, module string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		ns, member := raw[:i], raw[i+1:]
		for _, b := range bindings {
			if b.namespace && b.local == ns && member != "" {
				return member, b.module, true
			}
		}
		return "", "", false
	}
	for _, b := range bindings {
		if b.namespace || b.local != raw {
			continue
		}
		name := b.exportName
		if name == "" || name == "default" {
			name = b.local
		}
		return name, b.module, true
	}
	return "", "", false
}
