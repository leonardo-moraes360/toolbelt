🇺🇸 English | [🇧🇷 Português](../pt-BR/decisions/012-cli-framework-cobra.md)

# ADR-012: CLI Framework - Cobra

## Status

Accepted

## Context

toolbelt needs a robust command-line interface with:
- Hierarchical subcommands (`tb bench run`, `tb sandbox exec`)
- Global and per-command flags
- Automatic help generation
- Completions for bash/zsh/fish
- Support for configuration via file and environment variables

## Decision

We chose **Cobra** as the CLI framework.

### Alternatives Considered

| Framework | Pros | Cons |
|-----------|------|------|
| **Cobra** | De facto standard in Go (kubectl, hugo, gh), native completions, good docs | Verbose for simple cases |
| urfave/cli | Simpler API | Fewer features, smaller community |
| kingpin | Powerful parsing | Abandoned by maintainer |
| flag (stdlib) | Zero dependencies | No native subcommands |

### Rationale

1. **Maturity**: Used by kubectl, docker, hugo, gh - battle-tested
2. **Ecosystem**: Integration with Viper for config, automatic docs generation
3. **UX**: Native completions improve user productivity
4. **Maintenance**: Active community, maintained by Microsoft (via spf13)

## Consequences

### Positive
- Consistent command structure
- Automatic help and completions
- Makes adding new commands easier

### Negative
- External dependency (accepted given the benefit)
- Initial boilerplate

## Usage Example

```go
var benchCmd = &cobra.Command{
    Use:   "bench",
    Short: "Benchmark harness for agents",
    RunE:  runBenchmark,
}

benchCmd.Flags().StringP("agent", "a", "", "agent command")
benchCmd.Flags().Bool("fake", false, "use the fake agent")
```

## References

- [Cobra GitHub](https://github.com/spf13/cobra)
- [Cobra User Guide](https://cobra.dev/)
