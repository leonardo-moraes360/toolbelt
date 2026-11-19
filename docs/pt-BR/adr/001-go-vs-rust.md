[🇺🇸 English](../../adr/001-go-vs-rust.md) | 🇧🇷 Português

# ADR-001: Go como Linguagem Principal

## Status

accepted

## Contexto

O toolbelt precisa de uma linguagem para seu binário principal que atenda aos requisitos:

1. **Binário único estático**: distribuição simples sem dependências de runtime
2. **Concorrência nativa**: orquestrar múltiplos agentes, processos e I/O em paralelo
3. **Iteração rápida**: compilação rápida para ciclos curtos de desenvolvimento
4. **Ecossistema MCP**: suporte oficial ao protocolo MCP
5. **Performance adequada**: não é o gargalo (inferência é na GPU/API)

## Decisão

**Go** será a linguagem principal do toolbelt.

### Justificativas

1. **Binário estático**: `CGO_ENABLED=0 go build` produz binário sem dependências
2. **Goroutines e channels**: modelo de concorrência leve, ideal para orquestrar agentes
3. **Context**: propagação de cancelamento e timeouts nativa
4. **Compilação**: segundos, não minutos
5. **SDK MCP oficial**: [github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk)
6. **Ecossistema CLI**: cobra, bubbletea, lipgloss para CLIs e TUIs modernas
7. **Familiaridade**: linguagem mainstream, fácil de contribuir

## Consequências

### Positivas

- Deploy simples (um arquivo)
- Boa performance de I/O e concorrência
- Comunidade ativa, muitas bibliotecas
- Curva de aprendizado suave
- Tooling maduro (go vet, staticcheck, golangci-lint)

### Negativas

- Sem generics avançados (pode complicar abstrações)
- Garbage collector pode causar latência em cenários edge
- Menos expressivo que Rust para invariantes de tipo
- Error handling verboso

## Alternativas Consideradas

### Rust

**Por que considerado**:
- Performance máxima
- Segurança de memória garantida em compile-time
- Ecossistema emergente para LLM (mistral.rs, candle)

**Por que não escolhido**:
- A velocidade de inferência vem dos kernels GPU (llama.cpp, vLLM), não do cliente
- Rust só se paga se formos embedar inferência local, o que não é MVP
- Compilação lenta prejudica iteração
- Curva de aprendizado íngreme para contribuidores
- SDK MCP em Rust menos maduro

**Decisão futura**: se adicionarmos inferência local via mistral.rs/candle, componentes específicos podem ser escritos em Rust e chamados via FFI ou como processo separado.

### Python

**Por que considerado**:
- Ecossistema ML/AI dominante
- Prototipagem rápida

**Por que não escolhido**:
- Distribuição complexa (virtualenv, dependências)
- Performance de concorrência inferior (GIL)
- Não produz binário estático facilmente

### TypeScript/Node

**Por que considerado**:
- SDK MCP oficial
- Ecossistema web forte

**Por que não escolhido**:
- Runtime pesado (Node.js)
- Distribuição requer bundler ou Node instalado
- Concorrência menos previsível

## Referências

- [Go SDK MCP](https://github.com/modelcontextprotocol/go-sdk)
- [Discussão interna sobre linguagem] — não documentada formalmente
