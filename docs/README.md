# Prompt Engineering Studio (PES) — Documentación de Diseño

> **Estado: FASE DE DISEÑO — pendiente de revisión y aprobación.**
> No se ha escrito código de implementación. Este directorio contiene los 15 entregables
> solicitados antes de comenzar el desarrollo.

## Índice de entregables

| # | Documento | Contenido |
|---|-----------|-----------|
| 1 | [01-vision.md](01-vision.md) | Visión del producto, usuarios, propuesta de valor |
| 2 | [02-arquitectura.md](02-arquitectura.md) | Arquitectura completa (Clean Architecture, módulos, contratos) |
| 3 | [03-diagramas.md](03-diagramas.md) | Diagramas C4, flujo de datos, secuencia, ER (Mermaid) |
| 4 | [04-tecnologias.md](04-tecnologias.md) | Tecnologías recomendadas y justificación (Go + Python) |
| 5 | [05-estructura-carpetas.md](05-estructura-carpetas.md) | Estructura de carpetas del monorepo |
| 6 | [06-modelo-datos.md](06-modelo-datos.md) | Modelo de datos, esquema SQLite, formato de archivos |
| 7 | [07-sistema-plugins.md](07-sistema-plugins.md) | Sistema de plugins (WASM + procesos externos) |
| 8 | [08-roadmap.md](08-roadmap.md) | Roadmap MVP → v1 → v2 |
| 9 | [09-riesgos.md](09-riesgos.md) | Riesgos técnicos y mitigaciones |
| 10 | [10-plan-pruebas.md](10-plan-pruebas.md) | Plan de pruebas (unitarias, integración, E2E, rendimiento) |
| 11 | [11-diseno-ux.md](11-diseno-ux.md) | Diseño de UX (flujos, atajos, paneles) |
| 12 | [12-diseno-ui.md](12-diseno-ui.md) | Diseño de UI (layout, temas, componentes) |
| 13 | [13-persistencia.md](13-persistencia.md) | Estrategia de persistencia (offline-first) |
| 14 | [14-sincronizacion.md](14-sincronizacion.md) | Estrategia de sincronización futura |
| 15 | [15-plan-documentacion.md](15-plan-documentacion.md) | Plan de documentación |

## Resumen ejecutivo

**Prompt Engineering Studio (PES)** es un IDE de escritorio, offline-first, para crear,
organizar, validar, optimizar, probar y versionar prompts para cualquier LLM (local o remoto).
La analogía guía es: *lo que VS Code es para el código, PES lo es para los prompts*.

Decisiones clave (detalladas en los documentos):

- **Core Engine y Backend en Go** — binario único, rápido, multiplataforma, concurrencia nativa.
- **AI Engine en Python** — sidecar opcional (gRPC sobre socket local) que aporta el
  Optimizer, Benchmark avanzado y adaptadores a proveedores LLM. El sistema funciona al
  100 % sin él (IA estrictamente opcional).
- **UI de escritorio con Wails v2** — shell nativo Go + capa de vista web; toda la lógica
  vive en Go, la vista es una capa fina.
- **Persistencia dual**: prompts como archivos Markdown+YAML legibles y git-friendly
  (fuente de verdad) + índice SQLite (FTS5) para búsqueda instantánea sobre miles de prompts.
- **Plugins en dos niveles**: WASM sandboxeado (wazero) para extensiones seguras y
  procesos externos (JSON-RPC/stdio) para plugins Python o de cualquier lenguaje.
- **Sin telemetría obligatoria, datos locales por defecto, cifrado opcional** (age/XChaCha20).

## Proceso de aprobación

1. Revisar los 15 documentos.
2. Anotar objeciones o cambios (issue, PR review o comentarios).
3. Tras la aprobación explícita, se inicia la implementación siguiendo el
   [roadmap](08-roadmap.md), empezando por el hito **M0 (esqueleto + core engine)**.
