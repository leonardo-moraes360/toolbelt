🇺🇸 English | [🇧🇷 Português](../pt-BR/adr/004-sandbox-isolamento.md)

# ADR-004: Sandbox with bubblewrap + Landlock + seccomp

## Status

accepted

Implementation (v0.0.5): only the Landlock layer exists (`internal/sandbox`, fail-closed; outside Linux it returns `ErrUnsupported`). bubblewrap and seccomp are not implemented yet.

## Context

Coding agents execute arbitrary commands supplied by the language model. We need isolation to:

1. **Prevent damage**: limit writes to the working directory
2. **Block exfiltration**: deny network by default
3. **Limit resources**: avoid fork bombs, memory exhaustion
4. **Protect tests**: ensure benchmark files are not modified

Requirements:
- Linux (primary CI/dev environment)
- No full containers (overhead, complexity)
- Fine granularity (file, syscall)
- Acceptable performance

## Decision

Layered isolation stack:

1. **bubblewrap**: namespaces and bind-mounts
2. **Landlock**: runtime filesystem restrictions
3. **seccomp**: syscall filter
4. **cgroups v2**: CPU, RAM, time limits

```
┌─────────────────────────────────────────────────────────────────┐
│                      Isolated Process                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    seccomp                              │   │
│  │  Syscalls: read, write, open, ... (whitelist)           │   │
│  │  Blocked: ptrace, mount, reboot, ...                    │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Landlock                             │   │
│  │  /worktree: read+write                                  │   │
│  │  /tests: read-only                                      │   │
│  │  /tmp: read+write                                       │   │
│  │  Rest: blocked                                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   bubblewrap                            │   │
│  │  Namespaces: user, mount, net, pid                      │   │
│  │  Bind-mounts: worktree, tools                           │   │
│  │  Network: loopback only (or whitelist)                  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    cgroups v2                           │   │
│  │  CPU: 100% (or limited)                                 │   │
│  │  Memory: 1GB max                                        │   │
│  │  PIDs: 100 max                                          │   │
│  │  Timeout: kill after N seconds                          │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Responsibilities per Layer

| Layer | Responsibility | Configurable |
|-------|----------------|--------------|
| bubblewrap | Namespaces, mounts, network | --allow-net |
| Landlock | Runtime filesystem | --allow-read, --allow-write |
| seccomp | Syscalls | profile by task type |
| cgroups | Resources | --cgroup-cpu, --cgroup-mem |

## Consequences

### Positive

- **Defense in depth**: several independent layers
- **No root**: bubblewrap works without special privileges
- **Granular**: each layer can be relaxed as needed
- **Low overhead**: not a full container
- **Mainstream Linux**: technologies in the official kernel

### Negative

- **Linux only**: does not work natively on macOS/Windows
- **Complexity**: multiple technologies to understand
- **Hard debugging**: permission errors can be obscure
- **Recent Landlock**: kernel 5.13+, older distributions unsupported

## Alternatives Considered

### Docker/Containers

**Why considered**:
- Complete isolation
- Reproducible images
- Mature ecosystem

**Why not chosen**:
- Startup overhead
- Configuration complexity
- Requires a running daemon
- Overkill for simple command execution

### gVisor

**Why considered**:
- Full kernel isolation
- Container compatibility

**Why not chosen**:
- Significant performance overhead
- Deploy complexity
- Overkill for our case

### Namespaces only (unshare)

**Why considered**:
- Simple, native

**Why not chosen**:
- Insufficient filesystem granularity
- No syscall filter
- bubblewrap already does this better

### Firejail

**Why considered**:
- Easy to use
- Ready-made profiles

**Why not chosen**:
- Less fine-grained control
- Less programmatic configuration
- bubblewrap more suitable for embedding

## Sandbox Profiles

### Restrictive Profile (default)

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

### Profile with Network

```bash
bwrap \
  ... \
  --share-net \
  -- $COMMAND
# + iptables/nftables for host whitelist
```

### Profile for Tests

```bash
bwrap \
  ... \
  --ro-bind $TESTS $TESTS \  # tests are read-only
  --ro-bind $BENCHMARKS $BENCHMARKS \
  -- $COMMAND
```

## Integration with cgroups

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

## Fallback for macOS/Windows

For local development on macOS/Windows:

1. **Option 1**: Linux VM via Lima/WSL2
2. **Option 2**: Docker container as sandbox
3. **Option 3**: "Soft" sandbox (cwd restrict only, no real isolation)

Option 3 is acceptable for local dev, but CI and production must use Linux.

## References

- [bubblewrap](https://github.com/containers/bubblewrap)
- [Landlock](https://landlock.io/)
- [seccomp](https://www.kernel.org/doc/html/latest/userspace-api/seccomp_filter.html)
- [cgroups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)
