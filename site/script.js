(() => {
  const strings = {
    pt: {
      skip: "Ir para o conteúdo",
      menu: "Menu",
      "nav.problem": "Problema",
      "nav.commands": "Comandos",
      "nav.mcp": "MCP",
      "nav.install": "Instalar",
      "nav.docs": "Docs",
      "hero.eyebrow": "CLI · Go · MCP local",
      "hero.tag": "Cinto de ferramentas para agentes de código com IA",
      "hero.lede":
        "Um único binário tb que une verificação adversarial, revisão fresh-context, detecção de drift, anti-trapaça, memória de invariantes, grafo sob demanda e servidor MCP — rodando localmente, sob o mesmo comando e protocolo.",
      "hero.ctaInstall": "Instalar",
      "hero.ctaGithub": "Ver no GitHub",
      "hero.meta": "Apache-2.0 · Go 1.22+ · Linux / macOS / Windows · Fase 2",
      "hero.exampleNote": "Exemplo ilustrativo baseado na CLI real.",
      "problem.title": "O problema que resolve",
      "problem.body":
        "Agentes de código gastam tokens em grep cego, misturam implementação e avaliação no mesmo contexto e às vezes “passam” em testes sem cumprir o escopo. O tb concentra no mesmo cinto as peças que o README e a arquitetura descrevem: economizar tokens (grafo queryável, memória compacta), melhorar acurácia (sandbox, guard, separação autor/revisor) e acelerar o loop (execução local, stack compartilhado, harness de benchmark).",
      "problem.forAgentsLabel": "Agentes de IA",
      "problem.forAgents": "tools MCP e CLI no repo, sem serviço remoto obrigatório.",
      "problem.forHumansLabel": "Quem revisa o trabalho deles",
      "problem.forHumans":
        "verify, review, drift e guard com saída JSON e exit codes de CI.",
      "cmds.title": "Comandos e recursos",
      "cmds.lede":
        "Implementados na Fase 2. Saídas marcadas como exemplo; formato alinhado à CLI Cobra.",
      "cmds.verify":
        "Loop de verificação adversarial com rulers tests e visual (screenshot headless com skip gracioso).",
      "cmds.review":
        "Revisão fresh-context (Elephant-Goldfish) contra ticket/rd.md e invariantes; papéis scope/quality/acceptance.",
      "cmds.drift":
        "Drift de escopo / commit base / patch pinado; guard checa paths protegidos e padrões anti-trapaça no diff.",
      "cmds.graph":
        "Grafo sob demanda (ative com TB_GRAPH=on ou --enable); memória de invariantes fora da compactação.",
      "cmds.stack":
        "Empilha verify/review/drift/guard com diff, índice e memória compartilhados; um JSON agregado.",
      "cmds.bench":
        "Harness de benchmark (run/list/report/compare, inclusive A/B); init detecta toolchain; version reporta build.",
      "cmds.note":
        "tb run como comando de topo ainda é planejado (Fase 3+); hoje use tb sandbox run para isolamento Landlock no Linux. Também há tb workspace (worktrees) e tb mcp.",
      "mcp.title": "Servidor MCP",
      "mcp.lede":
        "tb mcp sobe um servidor MCP stateless sobre stdio (JSON-RPC 2.0). Roda localmente na sua máquina — sem hospedagem remota obrigatória.",
      "mcp.toolsTitle": "Tools (paridade CLI)",
      "mcp.graphNote":
        "Tools de grafo exigem TB_GRAPH=on no processo do servidor.",
      "install.title": "Instalação",
      "install.ledeBefore": "Prefira os binários das",
      "install.ledeLink": "Releases do GitHub",
      "install.ledeAfter":
        "(linux / darwin / windows × amd64 / arm64) e confira checksums.txt.",
      "install.goTitle": "Via Go",
      "install.goNote":
        "go install costuma embutir versão dev; use o Release para a tag.",
      "install.buildTitle": "Build local",
      "install.releases": "Abrir última release",
      "docs.title": "Documentação",
      "docs.lede":
        "Fontes no repositório — a CLI Cobra é a verdade quando houver divergência.",
      "docs.cli": "árvore de comandos",
      "docs.arch": "arquitetura",
      "docs.release": "releases",
      "docs.site": "este site / GitHub Pages",
      "docs.brand": "tema Synthwave",
      "docs.changelog": "histórico",
      "docs.readme": "visão geral e roadmap",
      "footer.tag": "Apache License 2.0 · Copyright 2026 Leonardo Moraes",
      copy: "Copiar",
      copied: "Copiado",
    },
    en: {
      skip: "Skip to content",
      menu: "Menu",
      "nav.problem": "Problem",
      "nav.commands": "Commands",
      "nav.mcp": "MCP",
      "nav.install": "Install",
      "nav.docs": "Docs",
      "hero.eyebrow": "CLI · Go · local MCP",
      "hero.tag": "A toolbelt for AI coding agents",
      "hero.lede":
        "One local tb binary that unifies adversarial verification, fresh-context review, drift detection, anti-cheat guards, invariant memory, on-demand graph search, and an MCP server — under one command and protocol.",
      "hero.ctaInstall": "Install",
      "hero.ctaGithub": "View on GitHub",
      "hero.meta": "Apache-2.0 · Go 1.22+ · Linux / macOS / Windows · Phase 2",
      "hero.exampleNote": "Illustrative example based on the real CLI.",
      "problem.title": "The problem it solves",
      "problem.body":
        "Coding agents burn tokens on blind grep, mix implementation and evaluation in the same context, and sometimes “pass” tests without meeting scope. tb packs the pieces described in the README and architecture: save tokens (queryable graph, compact memory), improve accuracy (sandbox, guard, author/reviewer separation), and speed up the loop (local execution, shared stack, benchmark harness).",
      "problem.forAgentsLabel": "AI agents",
      "problem.forAgents": "MCP tools and CLI in-repo, no remote service required.",
      "problem.forHumansLabel": "People who review their work",
      "problem.forHumans":
        "verify, review, drift, and guard with JSON output and CI exit codes.",
      "cmds.title": "Commands and features",
      "cmds.lede":
        "Shipped in Phase 2. Sample outputs are labeled as examples; format matches the Cobra CLI.",
      "cmds.verify":
        "Adversarial verification loop with tests and visual rulers (headless screenshot with graceful skip).",
      "cmds.review":
        "Fresh-context review (Elephant-Goldfish) against ticket/rd.md and invariants; scope/quality/acceptance roles.",
      "cmds.drift":
        "Scope / base-commit / pinned-patch drift; guard checks protected paths and anti-cheat patterns in the diff.",
      "cmds.graph":
        "On-demand graph (enable with TB_GRAPH=on or --enable); invariant memory kept outside compaction.",
      "cmds.stack":
        "Stacks verify/review/drift/guard with shared diff, index, and memory; one aggregated JSON.",
      "cmds.bench":
        "Benchmark harness (run/list/report/compare, including A/B); init detects toolchain; version reports the build.",
      "cmds.note":
        "Top-level tb run is still planned (Phase 3+); today use tb sandbox run for Landlock isolation on Linux. Also available: tb workspace (worktrees) and tb mcp.",
      "mcp.title": "MCP server",
      "mcp.lede":
        "tb mcp starts a stateless MCP server over stdio (JSON-RPC 2.0). It runs locally on your machine — no hosted service required.",
      "mcp.toolsTitle": "Tools (CLI parity)",
      "mcp.graphNote":
        "Graph tools require TB_GRAPH=on in the server process.",
      "install.title": "Install",
      "install.ledeBefore": "Prefer binaries from",
      "install.ledeLink": "GitHub Releases",
      "install.ledeAfter":
        "(linux / darwin / windows × amd64 / arm64) and verify checksums.txt.",
      "install.goTitle": "Via Go",
      "install.goNote":
        "go install usually embeds version dev; prefer the Release for the tagged build.",
      "install.buildTitle": "Local build",
      "install.releases": "Open latest release",
      "docs.title": "Documentation",
      "docs.lede":
        "Sources live in the repo — the Cobra CLI wins if docs diverge.",
      "docs.cli": "command tree",
      "docs.arch": "architecture",
      "docs.release": "releases",
      "docs.site": "this site / GitHub Pages",
      "docs.brand": "Synthwave theme",
      "docs.changelog": "history",
      "docs.readme": "overview and roadmap",
      "footer.tag": "Apache License 2.0 · Copyright 2026 Leonardo Moraes",
      copy: "Copy",
      copied: "Copied",
    },
  };

  const html = document.documentElement;
  const menuBtn = document.querySelector(".menu-btn");
  const mobileNav = document.getElementById("mobile-nav");
  const langButtons = document.querySelectorAll(".lang-btn");
  const copyButtons = document.querySelectorAll(".copy-btn");

  function applyLang(lang) {
    const dict = strings[lang] || strings.pt;
    html.lang = lang === "en" ? "en" : "pt-BR";
    document.querySelectorAll("[data-i18n]").forEach((el) => {
      const key = el.getAttribute("data-i18n");
      if (dict[key]) el.textContent = dict[key];
    });
    copyButtons.forEach((btn) => {
      if (!btn.classList.contains("is-copied")) {
        btn.textContent = dict.copy;
      }
    });
    langButtons.forEach((btn) => {
      const active = btn.getAttribute("data-lang") === lang;
      btn.classList.toggle("is-active", active);
      btn.setAttribute("aria-pressed", active ? "true" : "false");
    });
    try {
      localStorage.setItem("tb-lang", lang);
    } catch (_) {
      /* ignore */
    }
  }

  langButtons.forEach((btn) => {
    btn.addEventListener("click", () => applyLang(btn.getAttribute("data-lang")));
  });

  let saved = "pt";
  try {
    saved = localStorage.getItem("tb-lang") || "pt";
  } catch (_) {
    saved = "pt";
  }
  if (saved === "en") applyLang("en");

  if (menuBtn && mobileNav) {
    menuBtn.addEventListener("click", () => {
      const open = menuBtn.getAttribute("aria-expanded") === "true";
      menuBtn.setAttribute("aria-expanded", open ? "false" : "true");
      mobileNav.hidden = open;
    });
    mobileNav.querySelectorAll("a").forEach((link) => {
      link.addEventListener("click", () => {
        menuBtn.setAttribute("aria-expanded", "false");
        mobileNav.hidden = true;
      });
    });
  }

  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text);
    }
    return new Promise((resolve, reject) => {
      const area = document.createElement("textarea");
      area.value = text;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.left = "-9999px";
      document.body.appendChild(area);
      area.select();
      try {
        document.execCommand("copy");
        resolve();
      } catch (err) {
        reject(err);
      } finally {
        document.body.removeChild(area);
      }
    });
  }

  copyButtons.forEach((btn) => {
    btn.addEventListener("click", async () => {
      const lang = html.lang.startsWith("en") ? "en" : "pt";
      const dict = strings[lang];
      let text = btn.getAttribute("data-copy") || "";
      const target = btn.getAttribute("data-copy-target");
      if (target) {
        const node = document.querySelector(target);
        text = node ? node.textContent.trim() : text;
      }
      try {
        await copyText(text);
        btn.classList.add("is-copied");
        btn.textContent = dict.copied;
        window.setTimeout(() => {
          btn.classList.remove("is-copied");
          btn.textContent = dict.copy;
        }, 1600);
      } catch (_) {
        btn.textContent = "Error";
      }
    });
  });
})();
