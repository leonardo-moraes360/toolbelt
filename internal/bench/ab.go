package bench

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ABVariant: config + feature flags.
type ABVariant struct {
	Label string            `yaml:"label" json:"label"`
	Flags map[string]string `yaml:"flags,omitempty" json:"flags,omitempty"`
	Env   map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Agent AgentConfig       `yaml:"agent,omitempty" json:"agent,omitempty"`
}

func LoadABVariant(path string) (*ABVariant, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("lendo variante A/B: %w", err)
	}
	var v ABVariant
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parseando variante A/B: %w", err)
	}
	if v.Label == "" {
		v.Label = strings.TrimSuffix(baseName(path), ".yaml")
		v.Label = strings.TrimSuffix(v.Label, ".yml")
	}
	if v.Flags == nil {
		v.Flags = map[string]string{}
	}
	return &v, nil
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func ApplyVariant(agent *AgentConfig, variant *ABVariant) {
	if variant == nil {
		return
	}
	if variant.Agent.ID != "" {
		agent.ID = variant.Agent.ID
	}
	if variant.Agent.Command != "" {
		agent.Command = variant.Agent.Command
	}
	if variant.Agent.OutputParser != "" {
		agent.OutputParser = variant.Agent.OutputParser
	}
	if agent.Env == nil {
		agent.Env = map[string]string{}
	}
	for k, v := range variant.Env {
		agent.Env[k] = v
	}
	for k, v := range variant.Flags {
		agent.Env["TB_FLAG_"+strings.ToUpper(k)] = v
	}
	if variant.Label != "" {
		agent.Env["TB_AB_LABEL"] = variant.Label
	}
}

func TagResults(results []Result, variant *ABVariant) {
	if variant == nil {
		return
	}
	for i := range results {
		results[i].ConfigLabel = variant.Label
		if len(variant.Flags) > 0 {
			results[i].Flags = copyStringMap(variant.Flags)
		}
	}
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type ABComparison struct {
	LabelA         string  `json:"label_a"`
	LabelB         string  `json:"label_b"`
	PassAt1A       float64 `json:"pass_at_1_a"`
	PassAt1B       float64 `json:"pass_at_1_b"`
	DeltaPassAt1   float64 `json:"delta_pass_at_1"`
	AvgDurationA   float64 `json:"avg_duration_a"`
	AvgDurationB   float64 `json:"avg_duration_b"`
	DeltaDuration  float64 `json:"delta_duration_seconds"`
	AvgTokensA     float64 `json:"avg_tokens_a"`
	AvgTokensB     float64 `json:"avg_tokens_b"`
	DeltaTokensPct float64 `json:"delta_tokens_pct"`
	SuccessRateA   float64 `json:"success_rate_a"`
	SuccessRateB   float64 `json:"success_rate_b"`
	DeltaSuccess   float64 `json:"delta_success"`
}

func CompareAB(a, b *Summary) *ABComparison {
	if a == nil || b == nil {
		return nil
	}
	cmp := &ABComparison{
		LabelA:        a.AgentID,
		LabelB:        b.AgentID,
		PassAt1A:      a.Metrics.PassAt1,
		PassAt1B:      b.Metrics.PassAt1,
		DeltaPassAt1:  b.Metrics.PassAt1 - a.Metrics.PassAt1,
		AvgDurationA:  a.Metrics.AvgDuration,
		AvgDurationB:  b.Metrics.AvgDuration,
		DeltaDuration: b.Metrics.AvgDuration - a.Metrics.AvgDuration,
		AvgTokensA:    a.Metrics.AvgTokens,
		AvgTokensB:    b.Metrics.AvgTokens,
		SuccessRateA:  a.Metrics.PassAt1,
		SuccessRateB:  b.Metrics.PassAt1,
		DeltaSuccess:  b.Metrics.PassAt1 - a.Metrics.PassAt1,
	}
	if a.Metrics.AvgTokens > 0 {
		cmp.DeltaTokensPct = (b.Metrics.AvgTokens - a.Metrics.AvgTokens) / a.Metrics.AvgTokens * 100
	}
	return cmp
}
