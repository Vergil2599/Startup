# Prompt Engineering Studio (PES)

El IDE offline-first para Prompt Engineering: crear, componer, validar, optimizar,
probar, versionar y reutilizar prompts para cualquier LLM, local o remoto.
*Lo que VS Code es para el código, PES lo es para los prompts.*

## Estado del proyecto

**Fase de diseño.** La arquitectura completa está documentada y pendiente de revisión
y aprobación antes de escribir código de implementación.

➡️ **[Documentación de diseño (15 entregables)](docs/README.md)**

## Stack decidido (resumen)

- **Go** — core engine, storage, validación, CLI y shell de escritorio (Wails v2).
- **Python** — sidecar opcional de IA (`pes-ai`): optimizer, evaluación, adaptadores LLM.
- **Datos**: archivos Markdown+YAML (git/Obsidian-friendly) + índice SQLite FTS5 desechable.
- **Plugins**: WASM sandboxeado (wazero) + procesos JSON-RPC/stdio.
- **Principios**: offline-first, IA estrictamente opcional, sin telemetría, datos locales.
