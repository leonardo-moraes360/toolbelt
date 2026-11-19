package egm

import (
	"sort"
	"strings"
)

// configClass is the R4d group of a git config key.
type configClass int

const (
	configOther configClass = iota
	configNeutralized
	configFatal
)

// allowedExtensions are the repository extensions that change nothing an
// allowed subcommand could be tricked with; any other extensions.* is fatal.
var allowedExtensions = map[string]bool{
	"objectformat":    true,
	"worktreeconfig":  true,
	"refstorage":      true,
	"preciousobjects": true,
}

// neutralizedSections are sections overridden by the R4c -c options or never
// read by an allowed subcommand, whatever the subsection or name.
var neutralizedSections = map[string]bool{
	"filter":     true,
	"diff":       true,
	"mergetool":  true,
	"difftool":   true,
	"credential": true,
	"protocol":   true,
	"url":        true,
	"http":       true,
	"pager":      true,
}

// neutralizedCore are core.* keys the R4c -c options override or that no
// allowed subcommand reads.
var neutralizedCore = map[string]bool{
	"hookspath":            true,
	"fsmonitor":            true,
	"sshcommand":           true,
	"pager":                true,
	"editor":               true,
	"askpass":              true,
	"excludesfile":         true,
	"attributesfile":       true,
	"worktree":             true,
	"alternaterefscommand": true,
	"untrackedcache":       true,
	"gitproxy":             true,
}

// splitConfigKey splits section[.subsection].name. The subsection may hold
// dots, so the name is what follows the last one.
func splitConfigKey(key string) (section, sub, name string, hasSub bool) {
	k := strings.ToLower(key)
	first := strings.Index(k, ".")
	if first < 0 {
		return k, "", "", false
	}
	last := strings.LastIndex(k, ".")
	if first == last {
		return k[:first], "", k[last+1:], false
	}
	return k[:first], k[first+1 : last], k[last+1:], true
}

// classifyKey places one key in the fatal, neutralized or other group (R4d).
// The comparison is case-insensitive and <n> is any subsection.
func classifyKey(key string) configClass {
	section, _, name, hasSub := splitConfigKey(key)
	switch {
	case fatalKey(section, name, hasSub):
		return configFatal
	case section == "extensions":
		return configOther // an allowed extension
	case neutralizedKey(section, name, hasSub):
		return configNeutralized
	}
	return configOther
}

func fatalKey(section, name string, hasSub bool) bool {
	switch section {
	case "include", "includeif":
		return true
	case "extensions":
		return hasSub || !allowedExtensions[name]
	case "remote":
		return hasSub && name == "promisor"
	}
	return false
}

func neutralizedKey(section, name string, hasSub bool) bool {
	switch {
	case neutralizedSections[section]:
		return true
	case section == "core":
		return !hasSub && neutralizedCore[name]
	case section == "merge":
		return hasSub && name == "driver"
	case section == "submodule":
		return !hasSub && name == "recurse"
	}
	return false
}

// classifyConfig returns the sorted, deduplicated fatal and neutralized keys
// (lowercased, as git config --list prints section and name).
func classifyConfig(keys []string) (fatal, neutralized []string) {
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			continue
		}
		seen[k] = true
		switch classifyKey(k) {
		case configFatal:
			fatal = append(fatal, k)
		case configNeutralized:
			neutralized = append(neutralized, k)
		}
	}
	sort.Strings(fatal)
	sort.Strings(neutralized)
	return fatal, neutralized
}
