[🇺🇸 English](../TRANSLATION.md) | 🇧🇷 Português

# Tradução da documentação

O **inglês é a fonte de verdade**. As traduções vivem em `docs/<language_code>/`, espelhando o caminho em inglês; na raiz do repositório ficam só as docs em inglês. O português (pt-BR) está em `docs/pt-BR/` (inclusive `README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md` e `CHANGELOG.md`).

Ao alterar qualquer doc em inglês, atualize a tradução no mesmo PR. Tabela de mapeamento e regras (comandos/flags/saídas da CLI sem traduzir) em [docs/TRANSLATION.md](../TRANSLATION.md).

O teste `TestDocsTranslationMirrorComplete` falha se um `.md` em inglês (em `docs/` fora de `docs/<language_code>/`, ou doc da raiz) não tiver o par em `docs/pt-BR/`.
