package verify

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/leonardo-moraes360/toolbelt/internal/git"
)

const thresholdsPath = ".toolbelt/thresholds.yaml"

// Threshold sources reported in coverage.threshold_source.
const (
	ThresholdSourceBase = "base"
	ThresholdSourceFlag = "flag"
)

// thresholdsFile is the version 1 schema of .toolbelt/thresholds.yaml.
type thresholdsFile struct {
	Version  int `yaml:"version"`
	Coverage *struct {
		MinPercent *float64 `yaml:"min_percent"`
	} `yaml:"coverage"`
}

// BaseThresholds is what the base ref's thresholds file sets; a nil
// MinCoverage means no coverage floor.
type BaseThresholds struct {
	MinCoverage *float64
}

// LoadBaseThresholds reads .toolbelt/thresholds.yaml as committed at
// baseSHA (R4). The working tree is never read, so a change cannot lower its
// own floor. A missing file is no threshold; a malformed file, an
// out-of-range value or a symlink is an error, never "no threshold".
func LoadBaseThresholds(repoDir, baseSHA string) (BaseThresholds, error) {
	data, found, symlink, err := readBaseFile(repoDir, baseSHA, thresholdsPath)
	if err != nil {
		return BaseThresholds{}, err
	}
	if symlink {
		return BaseThresholds{}, fmt.Errorf("symlinked %s at base %s is not followed; refusing to judge coverage without a pinned threshold",
			thresholdsPath, shortSHA(baseSHA))
	}
	if !found {
		return BaseThresholds{}, nil
	}
	th, err := parseThresholds(data)
	if err != nil {
		return BaseThresholds{}, fmt.Errorf("%s malformed at base %s: %w", thresholdsPath, shortSHA(baseSHA), err)
	}
	return th, nil
}

func parseThresholds(data []byte) (BaseThresholds, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f thresholdsFile
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return BaseThresholds{}, errors.New("empty file (want version: 1)")
		}
		return BaseThresholds{}, err
	}
	if f.Version != 1 {
		return BaseThresholds{}, fmt.Errorf("version %d unsupported (want 1)", f.Version)
	}
	var th BaseThresholds
	if f.Coverage != nil && f.Coverage.MinPercent != nil {
		v := *f.Coverage.MinPercent
		if !validPercent(v) {
			return BaseThresholds{}, fmt.Errorf("coverage.min_percent %v outside [0, 100]", v)
		}
		th.MinCoverage = &v
	}
	return th, nil
}

// validPercent also rejects NaN, which fails both comparisons.
func validPercent(v float64) bool {
	return v >= 0 && v <= 100
}

// readBaseFile reads path as committed at baseRef. A symlink is reported,
// not followed: its target is not content verify can pin.
func readBaseFile(repoDir, baseRef, path string) (data []byte, found, symlink bool, err error) {
	// git show reports an unknown commit as a path missing from it, so the
	// commit is checked first: a bad base stays an error, never "missing".
	if _, err := git.Output(repoDir, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}"); err != nil {
		return nil, false, false, fmt.Errorf("base %s is not a commit: %w", baseRef, err)
	}
	mode, err := git.TreeMode(repoDir, baseRef, path)
	if err != nil {
		return nil, false, false, fmt.Errorf("reading %s from base %s: %w", path, baseRef, err)
	}
	if mode == git.ModeSymlink {
		return nil, false, true, nil
	}
	out, err := git.Output(repoDir, "show", "--no-textconv", baseRef+":"+path)
	if err == nil {
		return out, true, false, nil
	}
	if isMissingPathAtRef(err) {
		return nil, false, false, nil
	}
	return nil, false, false, fmt.Errorf("reading %s from base %s: %w", path, baseRef, err)
}
