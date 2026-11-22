[🇺🇸 English](../../adr/002-mcp-a2a-protocolos.md) | 🇧🇷 Português

# ADR-002: MCP + A2A como Protocolos de Comunicação

## Status

accepted. A seção "A2A para Agentes", o socket Unix do `tb serve` e o transporte HTTP/2 nunca foram construídos; o [ADR-020](020-agent-hierarchy-a2a.md) (proposto) os substitui: o tráfego agente ↔ agente é o `tb bus` local com envelope no formato A2A, o MCP é servido pelo `tb mcp` via stdio, e o `tb` não abre rede.

## Contexto

O toolbelt precisa de protocolos de comunicação para dois cenários:

1. **Agente ↔ Ferramentas**: como o modelo de linguagem invoca ferramentas do toolbelt
2. **Agente ↔ Agente**: como agentes na hierarquia (planner, supervisor, worker) se comunicam

Requisitos:
- Padrão aberto, não proprietário
- Suporte a múltiplos transportes (stdio, socket, HTTP)
- Serialização simples (JSON)
- Compatibilidade com ecossistema existente de IDEs e modelos

## Decisão

1. **MCP (Model Context Protocol)** para comunicação agente ↔ ferramentas
2. **A2A (Agent-to-Agent)** para comunicação agente ↔ agente
3. **JSON-RPC 2.0** como base comum para ambos

### MCP para Ferramentas

O `tb serve` expõe um servidor MCP que:
- Lista ferramentas disponíveis (`tb_run`, `tb_graph_search`, etc.)
- Recebe chamadas de ferramentas do modelo
- Retorna resultados estruturados
- Funciona via stdio (padrão) ou Unix socket

### A2A para Agentes

Comunicação entre níveis da hierarquia:
- Planner → Supervisor: delegação de tarefas
- Supervisor → Worker: atribuição de trabalho
- Worker → Supervisor: resultados e status
- Qualquer → Qualquer: queries de coordenação

### JSON-RPC 2.0 como Base

Ambos os protocolos usam JSON-RPC 2.0:

```json
{
  "jsonrpc": "2.0",
  "method": "tool/call",
  "params": {
    "name": "tb_run",
    "arguments": {"command": "go test"}
  },
  "id": 1
}
```

## Consequências

### Positivas

- **Interoperabilidade**: qualquer IDE/modelo com suporte MCP funciona
- **Padrão aberto**: não dependemos de vendor específico
- **Extensibilidade**: fácil adicionar novas ferramentas
- **Debugging**: JSON legível, fácil de inspecionar
- **Transportes flexíveis**: stdio para IDE, socket para processos internos

### Negativas

- **Overhead de serialização**: JSON não é o mais eficiente
- **A2A ainda emergente**: protocolo do Google, pode evoluir
- **Complexidade de gestão**: dois protocolos para manter

## Alternativas Consideradas

### gRPC

**Por que considerado**:
- Alta performance (protobuf binário)
- Streaming bidirecional
- Geração de código

**Por que não escolhido**:
- Complexidade adicional (compilador protoc)
- Não é padrão no ecossistema MCP
- Overhead para mensagens pequenas
- IDEs não suportam nativamente

### Protocolo Proprietário

**Por que considerado**:
- Controle total sobre design
- Otimizado para nosso caso de uso

**Por que não escolhido**:
- Reinventar a roda
- Sem suporte de ferramentas existentes
- Barreira para contribuidores

### Apenas MCP (sem A2A)

**Por que considerado**:
- Simplicidade (um protocolo só)

**Por que não escolhido**:
- MCP é otimizado para tool calling, não para coordenação de agentes
- A2A tem primitivas específicas: task assignment, status, delegation

## Arquitetura de Transportes

```
┌─────────────────────────────────────────────────────────────────┐
│                         Transportes                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  stdio (MCP padrão)          Unix Socket           HTTP/2      │
│  ─────────────────           ───────────           ──────      │
│  IDE ↔ tb serve              tb ↔ processos        remoto      │
│  Cursor, VSCode              internos, agentes     (futuro)    │
│                              na mesma máquina                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                       JSON-RPC 2.0                              │
│  ─────────────────────────────────────────────────────────────  │
│  {"jsonrpc":"2.0","method":"...","params":{...},"id":1}         │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│          MCP            │     │          A2A            │
│  ─────────────────────  │     │  ─────────────────────  │
│  tools/list             │     │  task/assign            │
│  tools/call             │     │  task/status            │
│  resources/read         │     │  task/complete          │
│  prompts/get            │     │  agent/query            │
└─────────────────────────┘     └─────────────────────────┘
```

## Referências

- [MCP Specification](https://spec.modelcontextprotocol.io/)
- [Google A2A Protocol](https://github.com/google/A2A)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
