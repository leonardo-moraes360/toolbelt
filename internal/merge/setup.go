package merge

import (
	"fmt"
	"strings"

	"github.com/leonardo-moraes360/toolbelt/internal/config"
)

// newResolver loads the user-only merge settings and builds the resolver for
// the semantic and prompt strategies. The repository's own config never
// contributes: its merge.* keys are reported as config_untrusted and ignored.
func newResolver(r runner, wt string, opts *Options, strategy Strategy, res *Result) (*resolver, func(), error) {
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
		r: r, wt: wt, maxRegionLines: user.Merge.Semantic.MaxRegionLines, maxFileBytes: user.Merge.Semantic.MaxFileBytes,
		sensitive: map[string]bool{},
	}
	for _, s := range res.Sensitive {
		rv.sensitive[s.Path] = true
	}
	closeFn := func() {}
	if strategy == StrategyPrompt {
		return nil, nil, fmt.Errorf("strategy prompt is not available yet")
	}
	return rv, closeFn, nil
}
