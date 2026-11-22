package config

import (
	"fmt"
	"io"
	"os"
	"reflect"
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
	if _, dup := warnedFiles.LoadOrStore(path, true); dup {
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
	fields := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		fields[name] = f.Type
	}
	var out []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		ft, ok := fields[k]
		if !ok {
			out = append(out, path)
			continue
		}
		out = append(out, unknownKeys(n.Content[i+1], ft, path)...)
	}
	return out
}
