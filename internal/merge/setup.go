package merge

import (
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// prepareSemantic loads the user-only merge settings and builds the resolver
// for the semantic and prompt strategies; the caller sets rv.wt once the
// temporary worktree exists. The repository's own config never contributes:
// its merge.* keys are reported as config_untrusted and ignored. For a prompt
// proposal it also opens the sandboxed agent (failing closed before any
// integration); for an apply it wires the receipt replay instead.
func prepareSemantic(r runner, opts *Options, strategy Strategy, prep []prepared, res *Result, rc *Receipt) (*resolver, func(), error) {
	user, err := config.LoadUser(opts.Config)
	if err != nil {
		return nil, nil, err
	}
	keys, kerr := config.RepoMergeKeys(r.repo)
	switch {
	case kerr != nil:
		res.Warnings = append(res.Warnings, Warning{Code: "config_untrusted", Message: ".toolbelt/config.yaml: " + kerr.Error()})
	case len(keys) > 0:
		res.Warnings = append(res.Warnings, Warning{Code: "config_untrusted",
			Message: "ignored repository keys " + strings.Join(keys, ", ") + " (set them in the user config)"})
	}
	rv := &resolver{
		r: r, maxRegionLines: user.Merge.Semantic.MaxRegionLines, maxFileBytes: user.Merge.Semantic.MaxFileBytes,
		sensitive: map[string]bool{},
	}
	for _, s := range res.Sensitive {
		rv.sensitive[s.Path] = true
	}
	closeFn := func() {}
	switch {
	case strategy == StrategyPrompt && rc != nil:
		rv.replay = &replay{rc: rc, used: map[string]bool{}, maxNovel: user.Merge.Prompt.MaxNovelLines}
		rv.hook = rv.replay.hook
	case strategy == StrategyPrompt:
		ag, closeAgent, err := openAgent(opts, user)
		if err != nil {
			return nil, nil, err
		}
		closeFn = closeAgent
		for i := range prep {
			ag.order = append(ag.order, prep[i].t.ID)
			ag.titles[prep[i].t.ID] = prep[i].t.Title
		}
		rv.agent = ag
		rv.hook = ag.agentHook
	}
	return rv, closeFn, nil
}
