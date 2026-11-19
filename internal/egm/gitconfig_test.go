package egm

import (
	"reflect"
	"testing"
)

func TestConfigClassifyKey(t *testing.T) {
	cases := map[string]configClass{
		"include.path":                  configFatal,
		"includeif.gitdir:/x/.path":     configFatal,
		"IncludeIf.onbranch:main.path":  configFatal,
		"extensions.partialclone":       configFatal,
		"extensions.partialClone":       configFatal,
		"extensions.frobnicate":         configFatal,
		"extensions.x.y":                configFatal,
		"remote.origin.promisor":        configFatal,
		"remote.a.b.promisor":           configFatal,
		"extensions.objectformat":       configOther,
		"extensions.worktreeConfig":     configOther,
		"extensions.refstorage":         configOther,
		"extensions.preciousobjects":    configOther,
		"core.hookspath":                configNeutralized,
		"core.fsmonitor":                configNeutralized,
		"core.sshCommand":               configNeutralized,
		"core.pager":                    configNeutralized,
		"core.editor":                   configNeutralized,
		"core.askpass":                  configNeutralized,
		"core.excludesfile":             configNeutralized,
		"core.attributesfile":           configNeutralized,
		"core.worktree":                 configNeutralized,
		"core.alternaterefscommand":     configNeutralized,
		"filter.x.clean":                configNeutralized,
		"filter.lfs.process":            configNeutralized,
		"diff.external":                 configNeutralized,
		"diff.x.textconv":               configNeutralized,
		"merge.npm.driver":              configNeutralized,
		"mergetool.x.cmd":               configNeutralized,
		"difftool.x.cmd":                configNeutralized,
		"credential.helper":             configNeutralized,
		"credential.https://x.helper":   configNeutralized,
		"protocol.allow":                configNeutralized,
		"url.https://x/.insteadof":      configNeutralized,
		"http.https://x/.extraheader":   configNeutralized,
		"pager.log":                     configNeutralized,
		"submodule.recurse":             configNeutralized,
		"core.bare":                     configOther,
		"core.repositoryformatversion":  configOther,
		"remote.origin.url":             configOther,
		"remote.origin.uploadpack":      configOther,
		"branch.main.vscode-merge-base": configOther,
		"merge.conflictstyle":           configOther,
		"submodule.x.url":               configOther,
		"core.x.hookspath":              configOther,
		"index.version":                 configOther,
		"singleword":                    configOther,
		"remote.promisor":               configOther,
		"merge.driver":                  configOther,
		"submodule.x.recurse":           configOther,
		"core.sparsecheckout":           configOther,
	}
	for key, want := range cases {
		if got := classifyKey(key); got != want {
			t.Errorf("classifyKey(%q) = %d, want %d", key, got, want)
		}
	}
}

func TestConfigClassifyDedupSorted(t *testing.T) {
	fatal, neut := classifyConfig([]string{"filter.x.clean", "include.path", "core.fsmonitor", "filter.x.clean", "core.bare", "extensions.foo"})
	if !reflect.DeepEqual(fatal, []string{"extensions.foo", "include.path"}) {
		t.Fatalf("fatal %v", fatal)
	}
	if !reflect.DeepEqual(neut, []string{"core.fsmonitor", "filter.x.clean"}) {
		t.Fatalf("neutralized %v", neut)
	}
}
