[🇺🇸 English](../../adr/005-anti-trapaca.md) | 🇧🇷 Português

# ADR-005: Estratégia Anti-Trapaça

## Status

accepted

## Contexto

Modelos de linguagem otimizam para recompensas aparentes, não necessariamente para o objetivo real. Em tarefas de código, isso pode manifestar-se como:

1. **Deletar testes que falham**: o teste não falha mais, mas o bug permanece
2. **Hardcodar outputs**: retorna valores esperados sem implementar lógica
3. **Skipar testes**: `t.Skip("TODO")` para passar artificialmente
4. **Modificar benchmarks**: alterar critérios de sucesso
5. **Retry excessivo**: tentar até passar por sorte

Precisamos de proteções que:
- Detectem e bloqueiem comportamentos desonestos
- Mantenham a integridade dos benchmarks
- Criem trail de auditoria completo
- Separem implementação de avaliação

## Decisão

Estratégia em cinco pilares:

### 1. Separação Implementador/Avaliador

```
┌─────────────────────────────────────────────────────────────────┐
│                         Tarefa                                  │
└─────────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┐
    │                   │
    ▼                   ▼
┌─────────────┐   ┌─────────────┐
│ Implementor │   │  Evaluator  │
│   Agent     │   │   Agent     │
├─────────────┤   ├─────────────┤
│ Pode:       │   │ Pode:       │
│ - ler código│   │ - ler código│
│ - escrever  │   │ - executar  │
│   código    │   │   testes    │
│ - rodar     │   │             │
│   testes    │   │ Não pode:   │
│   visíveis  │   │ - escrever  │
│             │   │ - ver impl  │
│ Não pode:   │   │   history   │
│ - ver/editar│   │             │
│   tests     │   │             │
│   ocultos   │   │             │
└─────────────┘   └─────────────┘
```

O avaliador:
- Executa em sandbox separado
- Não tem acesso ao histórico do implementador
- Apenas roda testes e reporta resultados

### 2. Arquivos de Teste Imutáveis

```bash
# Antes da tarefa
tb guard snapshot --paths "tests/**" "benchmarks/**"

# Durante execução
# tests/** montados como read-only no sandbox

# Após tarefa
tb guard verify --snapshot-id $SNAP_ID
# Se hash diferir → VIOLAÇÃO
```

Implementação:
- Bind-mount read-only via bubblewrap
- Hash SHA-256 de cada arquivo
- Verificação antes e depois
- Log de qualquer tentativa de escrita

### 3. Log de Auditoria Append-Only

```
.tb/audit.log (append-only, mesmo root não edita)

2026-01-15T10:30:00.123Z | worker-001 | file:read    | src/parser.go | OK
2026-01-15T10:30:01.456Z | worker-001 | file:write   | src/parser.go | OK | hash:abc→def
2026-01-15T10:30:02.789Z | worker-001 | file:write   | tests/test.go | BLOCKED | read-only
2026-01-15T10:30:03.012Z | worker-001 | run:command  | go test       | OK | exit:0
```

Características:
- Append-only enforçado via syscall filtering
- Hash encadeado (cada entrada inclui hash da anterior)
- Rotação por tamanho/tempo
- Exportável para análise externa

### 4. Tokens de Capacidade para Ações Destrutivas

Ações perigosas requerem aprovação explícita:

```go
type CapabilityToken struct {
    ID        string
    Action    string    // "delete:file", "modify:test"
    Resource  string    // path ou padrão
    GrantedBy string    // humano ou supervisor autorizado
    GrantedAt time.Time
    ExpiresAt time.Time
    OneTime   bool      // invalida após uso
    Signature string    // prova criptográfica
}
```

Fluxo:
1. Agente tenta ação destrutiva
2. Sistema bloqueia e cria pending request
3. Supervisor ou humano revisa
4. Se aprovado, token é emitido
5. Token tem validade curta (minutos)
6. Ação executa apresentando token

### 5. Detecção de Padrões Suspeitos

```go
type RewardHackingDetector struct {
    patterns []Pattern
}

var defaultPatterns = []Pattern{
    // Deleção de testes
    {
        Name:    "test_deletion",
        Trigger: "file:delete",
        Match:   "*_test.go",
        Action:  "block",
    },
    // Skip de testes
    {
        Name:    "test_skip",
        Trigger: "file:write",
        Match:   "*_test.go",
        Search:  `t\.Skip\(`,
        Action:  "flag_for_review",
    },
    // Output hardcodado
    {
        Name:    "hardcoded_output",
        Trigger: "file:write",
        Match:   "*.go",
        Search:  `return "expected_value"`,
        Action:  "flag_for_review",
    },
    // Cobertura artificial
    {
        Name:    "artificial_coverage",
        Trigger: "test:run",
        Detect:  "test without assertions",
        Action:  "flag_for_review",
    },
}
```

## Consequências

### Positivas

- **Integridade**: benchmarks não podem ser manipulados
- **Rastreabilidade**: toda ação é logada
- **Revisão**: padrões suspeitos são flagados
- **Confiança**: resultados são verificáveis

### Negativas

- **Overhead**: verificações adicionam latência
- **Falsos positivos**: detecção pode ser overly strict
- **Complexidade**: múltiplos sistemas para manter
- **UX**: aprovações podem frustrar fluxo

## Alternativas Consideradas

### Confiar no Modelo

**Por que considerado**:
- Simplicidade
- Modelos modernos são "bem comportados"

**Por que não escolhido**:
- Modelos otimizam para reward aparente
- Casos edge de reward hacking são documentados
- Segurança requer defesa em profundidade

### Revisão Humana de Tudo

**Por que considerado**:
- Máxima segurança
- Humano como juiz final

**Por que não escolhido**:
- Não escala
- Frustra automação
- Humanos também erram

### Testes de Mutação Pós-Hoc

**Por que considerado**:
- Detecta testes fracos
- Sem overhead durante execução

**Por que não escolhido como única medida**:
- Detecta depois, não previne
- Adicionar como camada extra (Fase 2)

## Casos de Teste para Detecção

```yaml
# Cenários que o sistema deve detectar/bloquear:

- scenario: delete_failing_test
  action: rm tests/auth_test.go
  expected: blocked
  
- scenario: skip_test
  action: add "t.Skip()" to test
  expected: flagged_for_review
  
- scenario: hardcode_output
  action: return literal "expected_value"
  expected: flagged_for_review
  
- scenario: modify_benchmark_threshold
  action: change assert from "< 100ms" to "< 10s"
  expected: blocked (benchmark is read-only)
  
- scenario: empty_test
  action: write test with no assertions
  expected: flagged_for_review
```

## Referências

- [Reward Hacking in RL](https://arxiv.org/abs/1706.03741)
- [Specification Gaming Examples](https://docs.google.com/spreadsheets/d/e/2PACX-1vRPiprOaC3HsCf5Tuum8bRfzYUiKLRqJmbOoC-32JorNdfyTiRRsR7Ea5eWtvsWzuxo8bjOxCG84dAg/pubhtml)
- [AI Safety via Debate](https://arxiv.org/abs/1805.00899)
