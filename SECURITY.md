# Política de Segurança

## Versões Suportadas

| Versão | Suportada |
| ------ | --------- |
| -      | Nenhuma versão lançada ainda |

Este projeto está em fase de design e documentação. Quando versões forem lançadas, esta tabela será atualizada.

## Reportando Vulnerabilidades

### Como Reportar

Use o **GitHub Private Vulnerability Reporting** para reportar vulnerabilidades de segurança:

1. Vá para a aba **Security** do repositório
2. Clique em **Report a vulnerability**
3. Preencha o formulário com detalhes da vulnerabilidade

**Não abra issues públicas para vulnerabilidades de segurança.**

### O Que Incluir no Relatório

- Descrição clara da vulnerabilidade
- Passos para reproduzir
- Impacto potencial
- Versão afetada (quando aplicável)
- Possíveis mitigações (se conhecidas)

### Tempo de Resposta Esperado

| Etapa | Tempo |
|-------|-------|
| Confirmação de recebimento | 48 horas |
| Triagem inicial | 7 dias |
| Correção (vulnerabilidades críticas) | 14 dias |
| Correção (vulnerabilidades médias/baixas) | 30 dias |

Estes tempos são metas, não garantias. Vulnerabilidades complexas podem levar mais tempo.

## Escopo

### Em Escopo

Vulnerabilidades relacionadas a:

- **Escape de sandbox**: qualquer forma de sair do isolamento bubblewrap/Landlock/seccomp
- **Bypass de guardrails**: contornar proteções anti-trapaça ou limites de capacidade
- **Escalação de privilégios**: agente obtendo permissões além do seu role
- **Exfiltração de dados**: vazamento de secrets ou dados protegidos
- **Injeção de comandos**: execução de comandos não autorizados
- **Corrupção de audit log**: modificar ou apagar registros de auditoria
- **Manipulação de benchmark**: formas de falsificar resultados de benchmark
- **Denial of Service**: crashar ou travar o sistema

### Fora de Escopo

- Vulnerabilidades em dependências (reporte diretamente aos mantenedores)
- Configurações incorretas pelo usuário
- Ataques que requerem acesso físico à máquina
- Social engineering
- Bugs sem impacto de segurança

## Processo de Disclosure

1. **Recebemos** o relatório via GitHub Private Vulnerability Reporting
2. **Confirmamos** recebimento em 48h
3. **Investigamos** e reproduzimos a vulnerabilidade
4. **Desenvolvemos** correção em branch privado
5. **Coordenamos** disclosure com o reporter
6. **Lançamos** correção
7. **Publicamos** advisory de segurança

## Créditos

Pesquisadores de segurança que reportam vulnerabilidades válidas serão creditados no advisory de segurança (a menos que prefiram anonimato).

## Compromisso

Nos comprometemos a:

- Não tomar ações legais contra pesquisadores agindo de boa fé
- Trabalhar com reporters para entender e resolver issues
- Manter reporters informados do progresso
- Creditar descobertas quando desejado

## Contato

Para questões sobre esta política (não para reportar vulnerabilidades), abra uma issue no repositório.
