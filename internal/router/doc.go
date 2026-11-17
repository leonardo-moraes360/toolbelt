// Package router implementa o scheduler em primeiro plano de tickets (Fase 3):
// pool de workers, entrega direta (afinidade ticket -> worker), retry e o
// watchdog embutido no loop (alertas, state.json, controle por arquivos).
//
// Layout em disco (tudo sob .toolbelt/router/, ignorado pelo git):
//
//	state.json         estado vivo do router (lido por tb router status / watchdog)
//	alerts.jsonl       alertas do watchdog (append-only)
//	control/           pedidos de controle (kill-<worker>.json)
//	logs/TKT-001.log   saída do agente por ticket
package router
