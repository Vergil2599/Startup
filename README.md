# Prompt Engineering Studio (PES)

El IDE offline-first para Prompt Engineering: crear, componer, validar, optimizar,
probar, versionar y reutilizar prompts para cualquier LLM, local o remoto.
*Lo que VS Code es para el código, PES lo es para los prompts.*

## Estado del proyecto

**Arquitectura aprobada — hito M0 implementado** (core engine + CLI, 100 % offline):

- ✅ Dominio, Template Engine (variables `{{x}}` con filtros, herencia de plantillas)
- ✅ Validation Engine (7 reglas deterministas, puntuación 0–100 explicable)
- ✅ Storage: archivos MD+YAML atómicos + índice SQLite FTS5 incremental
- ✅ Export (md/txt/json/yaml) e Import (PES-md/json/yaml/texto) con round-trip
- ✅ CLI `pes`: `init · new · list · search · render · validate · export · import · reindex`
- ✅ Presupuestos de rendimiento verificados en tests: 10 000 prompts →
  indexación ~6 s, búsqueda p95 ~22 ms, sync incremental ~174 ms

➡️ **[Documentación de diseño (15 entregables)](docs/README.md)** ·
[Roadmap](docs/08-roadmap.md) (siguiente: MVP v0.1, shell Wails + Prompt Builder)

## Desarrollo

```bash
go build ./cmd/pes        # compilar la CLI
go test ./...             # suite completa (incluye presupuestos de rendimiento)
go test -short ./...      # suite rápida

# Primer uso
./pes init -w ~/mis-prompts
./pes new "Mi primer prompt" -w ~/mis-prompts --tags demo
./pes validate <id> -w ~/mis-prompts
./pes export <id> -w ~/mis-prompts -f md
```

## Stack decidido (resumen)

- **Go** — core engine, storage, validación, CLI y shell de escritorio (Wails v2).
- **Python** — sidecar opcional de IA (`pes-ai`): optimizer, evaluación, adaptadores LLM.
- **Datos**: archivos Markdown+YAML (git/Obsidian-friendly) + índice SQLite FTS5 desechable.
- **Plugins**: WASM sandboxeado (wazero) + procesos JSON-RPC/stdio.
- **Principios**: offline-first, IA estrictamente opcional, sin telemetría, datos locales.
