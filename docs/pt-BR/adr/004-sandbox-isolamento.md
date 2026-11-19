[🇺🇸 English](../../adr/004-sandbox-isolamento.md) | 🇧🇷 Português

# ADR-004: Sandbox com bubblewrap + Landlock + seccomp

## Status

accepted

Implementação (v0.0.5): só a camada Landlock existe (`internal/sandbox`, fail-closed; fora do Linux retorna `ErrUnsupported`). bubblewrap e seccomp ainda não foram implementados.

## Contexto

Agentes de código executam comandos arbitrários fornecidos pelo modelo de linguagem. Precisamos de isolamento para:

1. **Prevenir danos**: limitar escrita ao diretório de trabalho
2. **Bloquear exfiltração**: negar rede por padrão
3. **Limitar recursos**: evitar fork bombs, memory exhaustion
4. **Proteger testes**: garantir que arquivos de benchmark não sejam modificados

Requisitos:
- Linux (ambiente primário de CI/dev)
- Sem containers completos (overhead, complexidade)
- Granularidade fina (arquivo, syscall)
- Performance aceitável

## Decisão

Stack de isolamento em camadas:

1. **bubblewrap**: namespaces e bind-mounts
2. **Landlock**: restrições de filesystem em runtime
3. **seccomp**: filtro de syscalls
4. **cgroups v2**: limites de CPU, RAM, tempo

```
┌─────────────────────────────────────────────────────────────────┐
│                      Processo Isolado                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    seccomp                              │   │
│  │  Syscalls: read, write, open, ... (whitelist)           │   │
│  │  Bloqueados: ptrace, mount, reboot, ...                 │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Landlock                             │   │
│  │  /worktree: read+write                                  │   │
│  │  /tests: read-only                                      │   │
│  │  /tmp: read+write                                       │   │
│  │  Resto: bloqueado                                       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   bubblewrap                            │   │
│  │  Namespaces: user, mount, net, pid                      │   │
│  │  Bind-mounts: worktree, tools                           │   │
│  │  Network: loopback only (ou whitelist)                  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    cgroups v2                           │   │
│  │  CPU: 100% (ou limitado)                                │   │
│  │  Memory: 1GB max                                        │   │
│  │  PIDs: 100 max                                          │   │
│  │  Timeout: kill after N seconds                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Responsabilidades por Camada

| Camada | Responsabilidade | Configurável |
|--------|------------------|--------------|
| bubblewrap | Namespaces, mounts, rede | --allow-net |
| Landlock | Filesystem runtime | --allow-read, --allow-write |
| seccomp | Syscalls | perfil por tipo de tarefa |
| cgroups | Recursos | --cgroup-cpu, --cgroup-mem |

## Consequências

### Positivas

- **Defesa em profundidade**: várias camadas independentes
- **Sem root**: bubblewrap funciona sem privilégios especiais
- **Granular**: cada camada pode ser relaxada conforme necessário
- **Baixo overhead**: não é container completo
- **Linux mainstream**: tecnologias no kernel oficial

### Negativas

- **Linux only**: não funciona em macOS/Windows nativamente
- **Complexidade**: múltiplas tecnologias para entender
- **Debugging difícil**: erros de permissão podem ser obscuros
- **Landlock recente**: kernel 5.13+, distribuições antigas não suportam

## Alternativas Consideradas

### Docker/Containers

**Por que considerado**:
- Isolamento completo
- Imagens reprodutíveis
- Ecossistema maduro

**Por que não escolhido**:
- Overhead de startup
- Complexidade de configuração
- Requer daemon rodando
- Overkill para execução de comandos simples

### gVisor

**Por que considerado**:
- Isolamento de kernel completo
- Compatibilidade com containers

**Por que não escolhido**:
- Performance overhead significativo
- Complexidade de deploy
- Overkill para nosso caso

### Apenas namespaces (unshare)

**Por que considerado**:
- Simples, nativo

**Por que não escolhido**:
- Granularidade insuficiente para filesystem
- Sem filtro de syscalls
- bubblewrap já faz isso melhor

### Firejail

**Por que considerado**:
- Fácil de usar
- Perfis prontos

**Por que não escolhido**:
- Menos controle fino
- Configuração menos programática
- bubblewrap mais adequado para embedding

## Perfis de Sandbox

### Perfil Restritivo (default)

```bash
bwrap \
  --unshare-all \
  --die-with-parent \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --bind $WORKTREE $WORKTREE \
  --ro-bind $TESTS $TESTS \
  --tmpfs /tmp \
  --dev /dev \
  --proc /proc \
  --unshare-net \
  -- $COMMAND
```

### Perfil com Rede

```bash
bwrap \
  ... \
  --share-net \
  -- $COMMAND
# + iptables/nftables para whitelist de hosts
```

### Perfil para Testes

```bash
bwrap \
  ... \
  --ro-bind $TESTS $TESTS \  # tests são read-only
  --ro-bind $BENCHMARKS $BENCHMARKS \
  -- $COMMAND
```

## Integração com cgroups

```go
type ResourceLimits struct {
    CPUPercent int           // 0-100
    MemoryMax  int64         // bytes
    PIDsMax    int           // max processes
    Timeout    time.Duration // kill after
}

func (s *Sandbox) Run(cmd string, limits ResourceLimits) error {
    cg := cgroups.New("tb-sandbox-" + uuid.New())
    defer cg.Destroy()
    
    cg.SetCPUMax(limits.CPUPercent)
    cg.SetMemoryMax(limits.MemoryMax)
    cg.SetPIDsMax(limits.PIDsMax)
    
    ctx, cancel := context.WithTimeout(context.Background(), limits.Timeout)
    defer cancel()
    
    return s.runInCgroup(ctx, cg, cmd)
}
```

## Fallback para macOS/Windows

Para desenvolvimento local em macOS/Windows:

1. **Opção 1**: VM Linux via Lima/WSL2
2. **Opção 2**: Container Docker como sandbox
3. **Opção 3**: Sandbox "soft" (apenas cwd restrict, sem isolamento real)

A opção 3 é aceitável para dev local, mas CI e produção devem usar Linux.

## Referências

- [bubblewrap](https://github.com/containers/bubblewrap)
- [Landlock](https://landlock.io/)
- [seccomp](https://www.kernel.org/doc/html/latest/userspace-api/seccomp_filter.html)
- [cgroups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
