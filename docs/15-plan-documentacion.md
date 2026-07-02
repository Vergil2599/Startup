# 15. Plan de documentación

## 15.1 Audiencias y artefactos

| Audiencia | Artefacto | Fuente | Cuándo |
|---|---|---|---|
| Usuarios | Guía de usuario + tutoriales por flujo | `docs/user/` (Markdown) | desde MVP |
| Usuarios | Ayuda integrada (tour de onboarding, tooltips, `F1` contextual) | strings i18n en la app | desde MVP |
| Autores de plugins | Guía de plugins + referencia del ABI/IR + `pes plugin new` | `docs/plugins/` + `proto/` | v1 |
| Contribuidores | Arquitectura (estos 15 docs), ADRs, CONTRIBUTING, guía de estilo | `docs/` + `docs/adr/` | ya |
| Integraciones | Referencia de la CLI y de los formatos de archivo | generada + `docs/formats/` | MVP |

## 15.2 Documentación automática (requisito del proyecto)

- **API Go**: `godoc` estándar; CI falla si un símbolo exportado carece de comentario
  (lint `revive/exported`). Publicación en pkg.go.dev para `pkg/pluginsdk`.
- **API Python (sidecar/SDK)**: docstrings + **mkdocstrings**; mismo gate en CI (ruff D).
- **CLI**: `cobra` genera la referencia de comandos en Markdown (`pes docs generate`)
  y las man pages; se regenera en cada release (drift = build roja).
- **Contratos gRPC/ABI**: `protoc-gen-doc` genera la referencia desde `proto/` — la
  documentación del contrato nunca puede divergir del contrato.
- **Formatos de archivo**: los esquemas del front-matter y de los YAML se definen como
  JSON Schema en `internal/…/schema/`; la referencia en docs se genera desde ellos y los
  mismos esquemas validan en runtime (una sola fuente de verdad).
- **Diagramas**: Mermaid en Markdown (renderizado nativo en GitHub/Obsidian); sin binarios
  de imagen que se desactualicen.

## 15.3 Sitio de documentación

- **MkDocs Material** sobre `docs/` (encaja con el stack Python ya presente), publicado
  con GitHub Pages en cada release; versionado con `mike` (docs por versión mayor/menor).
- Estructura del sitio: *Empezar* (instalación, primer prompt) → *Guías* (por función) →
  *Plugins* → *Referencia* (CLI, formatos, API) → *Arquitectura/Contribuir*.
- Idiomas: ES + EN desde v1 (los 15 docs de diseño solo en ES hasta estabilizarse).

## 15.4 Documentación como parte del Definition of Done

Una feature no se mergea sin:
1. Godoc/docstrings en la API pública que toca.
2. Actualización de la guía de usuario si cambia UX visible.
3. ADR si tomó una decisión de arquitectura (plantilla en `docs/adr/template.md`).
4. Changelog (formato *Keep a Changelog*, generado a partir de etiquetas de PR).

## 15.5 ADRs (Architecture Decision Records)

- Formato ligero: Contexto → Decisión → Consecuencias → Alternativas descartadas.
- Numerados e inmutables (se reemplazan con "superseded by", no se editan).
- Los 6 iniciales están listados en [02-arquitectura.md §2.8](02-arquitectura.md).

## 15.6 Mantenimiento

- Revisión de docs en cada release menor (checklist en el pipeline de release).
- Issues de documentación etiquetados `docs` con la misma prioridad que bugs de UX.
- Los ejemplos de código en docs se compilan/ejecutan en CI (extracción de bloques
  marcados) para que nunca queden obsoletos.
