[🇺🇸 English](../../decisions/012-cli-framework-cobra.md) | 🇧🇷 Português

# ADR-012: Framework CLI - Cobra

## Status

Aceita

## Contexto

O toolbelt precisa de uma interface de linha de comando robusta com:
- Subcomandos hierárquicos (`tb bench run`, `tb sandbox exec`)
- Flags globais e por-comando
- Geração automática de help
- Completions para bash/zsh/fish
- Suporte a configuração via arquivo e variáveis de ambiente

## Decisão

Escolhemos **Cobra** como framework CLI.

### Alternativas Consideradas

| Framework | Prós | Contras |
|-----------|------|---------|
| **Cobra** | Padrão de facto em Go (kubectl, hugo, gh), completions nativo, boa docs | Verboso para casos simples |
| urfave/cli | API mais simples | Menos features, comunidade menor |
| kingpin | Parsing poderoso | Abandonado pelo mantenedor |
| flag (stdlib) | Zero dependências | Sem subcomandos nativos |

### Justificativa

1. **Maturidade**: Usado por kubectl, docker, hugo, gh - battle-tested
2. **Ecossistema**: Integração com Viper para config, geração automática de docs
3. **UX**: Completions nativos melhoram produtividade do usuário
4. **Manutenção**: Comunidade ativa, mantido pela Microsoft (via spf13)

## Consequências

### Positivas
- Estrutura consistente de comandos
- Help e completions automáticos
- Facilita adicionar novos comandos

### Negativas
- Dependência externa (aceito dado o benefício)
- Boilerplate inicial

## Exemplo de Uso

```go
var benchCmd = &cobra.Command{
    Use:   "bench",
    Short: "Harness de benchmark para agentes",
    RunE:  runBenchmark,
}

benchCmd.Flags().StringP("agent", "a", "", "comando do agente")
benchCmd.Flags().Bool("fake", false, "usa agente fake")
```

## Referências

- [Cobra GitHub](https://github.com/spf13/cobra)
- [Cobra User Guide](https://cobra.dev/)
