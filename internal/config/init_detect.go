package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// InitOptions are the tb init switches that change what detection writes.
type InitOptions struct {
	// WriteChecks writes the suggested named checks into verify.checks;
	// without it they are only reported (R3: suggest, never add silently).
	WriteChecks bool
}

// InitResult reports what InitWithOptions detected.
type InitResult struct {
	Test DetectedCmd
	// SuggestedChecks are the checks detection found for this repository,
	// written or not.
	SuggestedChecks []VerifyCheck
	// ChecksWritten is true when SuggestedChecks went into a new config.yaml.
	ChecksWritten bool
	// Created is false when config.yaml already existed and was left as is.
	Created bool
}

// InitWithOptions creates the config dir and, when config.yaml is missing,
// writes it with the detected verify commands. An existing config.yaml is
// never rewritten; checks are still detected so the caller can suggest them.
func InitWithOptions(dir string, opts InitOptions) (InitResult, error) {
	var res InitResult
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, fmt.Errorf("creating config dir: %w", err)
	}

	for _, sub := range []string{"vault", "logs", "bench"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return res, fmt.Errorf("creating %s dir: %w", sub, err)
		}
	}

	if root, ok := projectRootForInit(dir); ok {
		res.SuggestedChecks = DetectSuggestedChecks(root)
	}

	configPath := filepath.Join(dir, DefaultConfigFile)
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		return res, nil
	}

	cfg := DefaultConfig()
	// Left out so a repository config written by init never sets user-only
	// keys (which would draw config_untrusted warnings); defaults apply.
	cfg.Graph.Vector, cfg.Graph.Embed = GraphVectorConfig{}, GraphEmbedConfig{}
	res.Test = applyDetectedVerifyCmds(cfg, dir)
	if opts.WriteChecks && len(res.SuggestedChecks) > 0 {
		cfg.Verify.Checks = append([]VerifyCheck(nil), res.SuggestedChecks...)
		res.ChecksWritten = true
	}
	if err := Save(cfg, configPath); err != nil {
		return res, fmt.Errorf("saving default config: %w", err)
	}
	res.Created = true
	return res, nil
}

func applyDetectedVerifyCmds(cfg *Config, dir string) DetectedCmd {
	var detected DetectedCmd
	root, ok := projectRootForInit(dir)
	if !ok {
		return detected
	}
	detected = DetectTestCmdDetailed(root)
	cfg.Verify.TestCmd = detected.Cmd
	if cmd := DetectBuildCmd(root); cmd != "" {
		cfg.Verify.BuildCmd = cmd
	}
	if cmd := DetectTypecheckCmd(root); cmd != "" {
		cfg.Verify.TypecheckCmd = cmd
	}
	if cmd := DetectLintCmd(root); cmd != "" {
		cfg.Verify.LintCmd = cmd
	}
	if detected.Cmd == "" && detected.Warning == "" {
		detected.Warning = "no test toolchain detected; verify.test_cmd left empty (does not use go test without go.mod)"
	}
	return detected
}
