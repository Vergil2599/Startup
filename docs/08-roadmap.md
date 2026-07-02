# 8. Roadmap por versiones

Principio: cada hito termina en algo **usable y probado**, no en capas horizontales
inconexas. El validador, la biblioteca y el builder llegan antes que cualquier IA.

## M0 — Fundaciones (interno, ~semanas 1–4)

- Esqueleto del monorepo, Taskfile, CI (lint + test + build matrix 3 SOs).
- `internal/domain` completo con tests.
- Template Engine: parser `{{var}}`, filtros básicos, resolución por ámbitos.
- Storage: repo FS (MD+YAML atómico) + índice SQLite FTS5 + reindex incremental.
- CLI mínima: `pes init`, `pes list`, `pes render`, `pes validate`, `pes reindex`.
- **Criterio de salida**: crear/renderizar/buscar 10 000 prompts sintéticos por CLI,
  búsqueda < 50 ms p95.

## MVP (v0.1–v0.4) — "El editor que ya es útil offline"

| Versión | Contenido |
|---|---|
| v0.1 | Shell Wails; **Prompt Builder** por bloques (activar/desactivar, reordenar, drag&drop); autoguardado; undo/redo; temas claro/oscuro |
| v0.2 | **Biblioteca**: carpetas, tags, favoritos, búsqueda instantánea; vista previa en tiempo real con variables resueltas |
| v0.3 | **Plantillas** builtin por categoría + herencia; **variables** globales/proyecto/prompt; import/export **MD, JSON, YAML, TXT, clipboard** |
| v0.4 | **Validador** completo con puntuación en vivo; **historial** (snapshots, restaurar); atajos de teclado; paleta de comandos |

**Criterio de salida del MVP**: un usuario nuevo crea desde plantilla, edita por bloques,
valida, versiona y exporta un prompt en < 5 minutos, sin red. Cero features de IA.

## v1.0 — "El estudio completo"

- **Composer** (capas + estrategias de merge) y **Comparador** (diff textual + estructural, estadísticas).
- **Simulador**: providers Go nativos (Ollama, LM Studio, OpenAI-compatible, llama.cpp server);
  ejecución, guardado de respuestas, comparación lado a lado.
- **Sidecar `pes-ai`** (Python): **Optimizer** opcional (claridad, redundancia, estructura,
  ejemplos) con diff de sugerencias aplicables bloque a bloque; instalación guiada; todo desactivable.
- **Sistema de plugins v1**: WASM + JSON-RPC, extension points `llm.provider`,
  `export.codec`, `import.codec`, `validation.rule`; SDKs Go y Python; `pes plugin verify`.
- Export **HTML y PDF**; cifrado opcional del workspace; backups automáticos rotados.
- Paneles acoplables y editor dividido.
- Documentación de usuario y de autores de plugins publicada.

**Criterio de salida**: API de plugins declarada estable (1.0); un tercero puede escribir
un provider en Python en < 1 hora con la guía.

## v2.0 — "La plataforma"

- **Benchmark** multi-modelo: matriz modelos × repeticiones; métricas de tiempo, tokens,
  costo, consistencia; calidad opcional por rúbrica/LLM-judge; informes exportables.
- **Integraciones** (como plugins first-party): Obsidian (workspace compartido),
  Git/GitHub (commit/push de prompts, PRs), **MCP server** (exponer la biblioteca a agentes),
  Claude Code / Gemini CLI / Cursor / OpenClaw (export dirigido + comandos), VS Code (extensión puente).
- Extension points de UI declarativa (`panel`) y `block.type` custom.
- **Marketplace** de plugins y paquetes de plantillas con firmas.
- **Sincronización opcional** (ver [14-sincronizacion.md](14-sincronizacion.md)): git-backed
  primero; E2E-encrypted como investigación.
- Rendimiento: perfilado con workspaces de 50 000 prompts.

## Política transversal

- Semver; rama estable + betas etiquetadas.
- Presupuestos de rendimiento en CI (arranque, búsqueda, indexación) — regresión = build roja.
- Ninguna feature de IA entra si su ausencia rompe un flujo offline.
- Cada versión menor incluye migradores de esquema probados con workspaces reales de la anterior.
