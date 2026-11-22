package guard

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ViolationVerifyConfigChanged flags a change to the verify: section of the
// repo config. Verify reads its commands from the base ref (R0), so an edit
// here is the change trying to rewrite the gate that judges it.
const ViolationVerifyConfigChanged ViolationType = "verify_config_changed"

const repoConfigPath = ".toolbelt/config.yaml"

// checkVerifyConfig compares the verify: section at base and head by reading
// both blobs directly, so it holds even when the caller supplies its own
// name-status diff. A malformed head file is a finding (fail-closed: an
// unparsable section cannot be shown unchanged); a malformed base file is an
// error, matching how guard.yaml at base is treated.
func (c *Checker) checkVerifyConfig(base, head string) ([]Violation, error) {
	baseSec, baseOK, err := c.verifySection(base)
	if err != nil {
		var pe *verifyParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s malformed at base %s: %w", repoConfigPath, base, pe.err)
		}
		return nil, err
	}
	headSec, headOK, err := c.verifySection(head)
	if err != nil {
		var pe *verifyParseError
		if !errors.As(err, &pe) {
			return nil, err
		}
		return []Violation{{
			Type: ViolationVerifyConfigChanged, File: repoConfigPath, Severity: SeverityError,
			Message: "verify config unreadable at head (malformed YAML): verify",
			Context: "verify",
		}}, nil
	}
	if !baseOK && !headOK {
		return nil, nil
	}
	keys := changedVerifyKeys(baseSec, headSec, baseOK, headOK)
	if len(keys) == 0 {
		return nil, nil
	}
	list := strings.Join(keys, ", ")
	return []Violation{{
		Type: ViolationVerifyConfigChanged, File: repoConfigPath, Severity: SeverityError,
		Message: "verify config changed: " + list,
		Context: list,
	}}, nil
}

type verifyParseError struct{ err error }

func (e *verifyParseError) Error() string { return e.err.Error() }

func (e *verifyParseError) Unwrap() error { return e.err }

// verifySection returns the verify: mapping at ref; ok is false when the file
// or the section is absent. A null section counts as absent.
func (c *Checker) verifySection(ref string) (sec map[string]any, ok bool, err error) {
	data, err := c.gitShow(ref, repoConfigPath)
	if err != nil {
		if isMissingGitObject(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading %s from %s: %w", repoConfigPath, ref, err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal([]byte(data), &raw); err != nil {
		return nil, false, &verifyParseError{err: err}
	}
	v, present := raw["verify"]
	if !present || v == nil {
		return nil, false, nil
	}
	m, isMap := v.(map[string]any)
	if !isMap {
		// A scalar or list where a mapping belongs is still a section; keep it
		// under a sentinel key so any change to it is reported.
		return map[string]any{"": v}, true, nil
	}
	return m, true, nil
}

// changedVerifyKeys lists dotted top-level keys under verify: whose values
// differ. Adding or removing a whole section reports its keys, or "verify"
// when the section is empty.
func changedVerifyKeys(base, head map[string]any, baseOK, headOK bool) []string {
	seen := map[string]struct{}{}
	for k := range base {
		seen[k] = struct{}{}
	}
	for k := range head {
		seen[k] = struct{}{}
	}
	var keys []string
	for k := range seen {
		bv, bok := base[k]
		hv, hok := head[k]
		if bok == hok && reflect.DeepEqual(bv, hv) {
			continue
		}
		if k == "" {
			keys = append(keys, "verify")
			continue
		}
		keys = append(keys, "verify."+k)
	}
	sort.Strings(keys)
	if len(keys) == 0 && baseOK != headOK {
		keys = []string{"verify"}
	}
	return keys
}
