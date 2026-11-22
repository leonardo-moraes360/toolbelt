package config

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// warnOut receives the unknown-key warnings; tests swap it.
var warnOut io.Writer = os.Stderr

// warnedFiles keeps one report per file per process: Load runs several times
// in a single command and each run would repeat the same lines.
var warnedFiles sync.Map

// warnUnknownKeys prints one stderr line per key of a config file that Config
// does not know, with its full path. It never fails: an old file with a
// removed key keeps loading and the key is ignored.
func warnUnknownKeys(path string, data []byte) {
	key := path
	if abs, err := filepath.Abs(path); err == nil {
		key = abs
		if resolved, err := filepath.EvalSymlinks(abs); err == nil {
			key = resolved
		}
	}
	if _, dup := warnedFiles.LoadOrStore(key, true); dup {
		return
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return
	}
	for _, key := range unknownKeys(doc.Content[0], reflect.TypeOf(Config{}), "") {
		fmt.Fprintf(warnOut, "warning: %s: unknown config key %s (ignored)\n", path, key)
	}
}

// unknownKeys walks a YAML node against a struct type by its yaml tags.
// Maps and types with their own unmarshaler are not checked below their level.
func unknownKeys(n *yaml.Node, t reflect.Type, prefix string) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch {
	case t.Kind() == reflect.Slice && n.Kind == yaml.SequenceNode:
		var out []string
		for _, item := range n.Content {
			out = append(out, unknownKeys(item, t.Elem(), prefix)...)
		}
		return out
	case t.Kind() != reflect.Struct || n.Kind != yaml.MappingNode:
		return nil
	}
	if reflect.PointerTo(t).Implements(reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()) {
		return nil
	}
	fields := yamlFields(t)
	var out []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Tag == "!!merge" {
			for _, m := range mergeTargets(v) {
				out = append(out, unknownKeys(m, t, prefix)...)
			}
			continue
		}
		path := k.Value
		if prefix != "" {
			path = prefix + "." + k.Value
		}
		ft, ok := fields[k.Value]
		if !ok {
			out = append(out, path)
			continue
		}
		out = append(out, unknownKeys(v, ft, path)...)
	}
	return out
}

// mergeTargets resolves the value of a `<<` key: one alias or a sequence of
// them (a literal mapping is accepted too).
func mergeTargets(v *yaml.Node) []*yaml.Node {
	switch v.Kind {
	case yaml.AliasNode:
		return []*yaml.Node{v.Alias}
	case yaml.SequenceNode:
		var out []*yaml.Node
		for _, item := range v.Content {
			out = append(out, mergeTargets(item)...)
		}
		return out
	}
	return []*yaml.Node{v}
}

// yamlFields maps yaml key names to field types; `,inline` structs
// contribute their own fields, `-` and unexported fields none.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("yaml")
		if tag == "-" || (!f.IsExported() && !f.Anonymous) {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if slices.Contains(strings.Split(opts, ","), "inline") {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				maps.Copy(fields, yamlFields(ft))
			}
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		fields[name] = f.Type
	}
	return fields
}
