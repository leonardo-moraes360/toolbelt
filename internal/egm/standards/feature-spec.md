# Standard: feature-spec

A feature spec explains what will be built, why, and how, in enough detail to
review the decision and to implement it.

Required headings (either language satisfies each line; the whole heading line
must match):

| Section | English | Portuguese |
|---------|---------|------------|
| Overview | `## Overview` | `## Visão geral` |
| Problem | `## Problem` | `## Problema` |
| Goals | `## Goals` | `## Objetivos` |
| Non-goals | `## Non-Goals` | `## Não-objetivos` |
| Technical plan | `## Technical Plan` | `## Plano técnico` |

Criteria:
- Overview: one or two paragraphs a newcomer can read to know what changes and
  for whom.
- Problem: the current behavior or gap, with concrete evidence (an incident, a
  measurement, a user journey), not only the desired solution.
- Goals: outcomes that can be checked; each goal is served by at least one
  requirement.
- Non-goals: what is deliberately left out, so reviewers do not ask for it and
  implementers do not build it.
- Technical plan: numbered requirements, each with one trigger and one
  observable behavior; failure and edge cases stated; interfaces (commands,
  flags, files, schemas, exit codes) named exactly.
- Terms used in requirements are defined once (a glossary when there are many).
- Alternatives that were considered are listed with the reason for rejecting
  them.
- Open questions are explicit and do not hide decisions the requirements
  depend on.
- When the spec has a detailed implementation plan, every file it creates or
  changes is listed with an action and a reason tied to a requirement.
