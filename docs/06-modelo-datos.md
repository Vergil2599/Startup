# 6. Modelo de datos

Dos representaciones coordinadas: **archivos** (fuente de verdad, legibles, git-friendly)
y **SQLite** (índice desechable para búsqueda/consulta). Este documento define ambas.

## 6.1 Entidades del dominio

### Prompt
| Campo | Tipo | Notas |
|---|---|---|
| `id` | ULID | estable, generado al crear |
| `title` | string | obligatorio |
| `description` | string | opcional |
| `folder` | path relativo | derivado de la ubicación del archivo |
| `tags` | []string | libres, normalizados a minúsculas |
| `category` | string | de la taxonomía de plantillas |
| `favorite` | bool | |
| `template_id` | ULID? | plantilla de origen (si aplica) |
| `blocks` | []Block | ordenados |
| `variables_local` | map[string]string | ámbito prompt |
| `created_at / updated_at` | RFC3339 | |
| `schema` | int | versión del esquema de archivo (migraciones) |

### Block
| Campo | Tipo | Notas |
|---|---|---|
| `type` | BlockType | `role, objective, context, background, constraints, rules, workflow, input, output, examples, failure_conditions, success_criteria, notes` + tipos custom de plugins |
| `enabled` | bool | activable/desactivable sin borrar contenido |
| `content` | Markdown | puede contener `{{variables}}` |
| `order` | int | posición en el prompt |
| `meta` | map | extensible (p. ej. peso en validación) |

### Template
Igual que Prompt, más: `extends: template_id?` (herencia simple, un nivel de padre;
la resolución es recursiva hasta la raíz, con detección de ciclos), y por bloque
una política `inherit | override | append`.

### Variable
`name` (snake_case validado), `value`, `description?`, `scope` (`global | project | prompt`),
`secret: bool` (los secretos se guardan en keyring/age, nunca en YAML plano).

### Version (historial)
`id`, `prompt_id`, `seq`, `author` (de settings/git), `comment`, `created_at`,
`content_hash` (SHA-256), `snapshot_ref` (archivo content-addressed en `.pes/history/`).
El diff se calcula bajo demanda (texto Myers + diff estructural por bloques);
no se almacenan diffs, solo snapshots deduplicados por hash.

### Composition
`id`, `title`, `layers: []{ref: prompt_id|template_id, order, enabled}`,
`merge_strategy` (`concat | by_block_type` — en `by_block_type` los bloques del mismo
tipo se fusionan según política `append | replace`), `output_prompt_id?` (si se
materializó como prompt).

### RunResult / BenchmarkRun
`RunResult`: `id`, `prompt_id`, `prompt_version_hash`, `provider`, `model`, `params`
(temperature…), `rendered_prompt_hash`, `response`, `latency_ms`, `tokens_in/out`,
`cost_estimate?`, `error?`, `created_at`.
`BenchmarkRun`: `id`, `prompt_id`, `models[]`, `reps`, agregados por modelo
(`p50/p95 latencia, tokens, costo, consistency_score, quality_score?`), `run_ids[]`.

### ValidationReport (efímero, cacheado)
`prompt_id`, `content_hash`, `score 0–100`, `breakdown` (por categoría),
`findings[]: {rule_id, severity(info|warn|error), block_type?, message, suggestion?}`.
Se cachea por `content_hash`; no forma parte de la fuente de verdad.

## 6.2 Formato de archivo: prompt (`*.md`)

```markdown
---
pes: 1                      # versión de esquema
id: 01J8ZK7Q3W9X2Y4V5B6N7M8P9R
title: Revisor de código Go
description: Revisión con estándares del equipo
tags: [go, code-review, calidad]
category: software-development
favorite: true
template: 01J8ZJXQ...        # opcional
variables:
  language: Go
blocks:                      # orden + estado; el contenido vive abajo
  - {type: role,        enabled: true}
  - {type: objective,   enabled: true}
  - {type: constraints, enabled: false}
created: 2026-07-02T10:00:00Z
updated: 2026-07-02T12:30:00Z
---

## @role
Actúa como revisor senior de {{language}}…

## @objective
Detectar defectos de corrección y proponer mejoras…

## @constraints
No proponer reescrituras completas…
```

- Los encabezados `## @tipo` delimitan bloques → el archivo es legible en
  GitHub/Obsidian tal cual, y parseable sin ambigüedad.
- Escrituras atómicas (tmp + rename). Un prompt = un archivo.
- Plantillas: mismo formato en `templates/` con `extends:` opcional.
- Variables: YAML plano (`variables/global.yaml`, `variables/<proyecto>.yaml`).
- Composiciones: YAML (`compositions/*.yaml`).

## 6.3 Esquema SQLite (índice desechable)

```sql
PRAGMA user_version = 1;             -- migraciones del índice

CREATE TABLE prompts (
  id TEXT PRIMARY KEY, path TEXT UNIQUE NOT NULL,
  title TEXT NOT NULL, description TEXT, category TEXT,
  folder TEXT, favorite INTEGER DEFAULT 0,
  template_id TEXT, content_hash TEXT NOT NULL,
  score INTEGER,                     -- última puntuación de validación
  created_at TEXT, updated_at TEXT, mtime INTEGER  -- para re-indexado incremental
);
CREATE TABLE tags (prompt_id TEXT, tag TEXT, PRIMARY KEY (prompt_id, tag));
CREATE INDEX idx_tags_tag ON tags(tag);

CREATE VIRTUAL TABLE prompts_fts USING fts5(
  title, description, body, tags,
  content='', tokenize='unicode61 remove_diacritics 2'
);

CREATE TABLE versions (
  id TEXT PRIMARY KEY, prompt_id TEXT NOT NULL, seq INTEGER,
  author TEXT, comment TEXT, content_hash TEXT, created_at TEXT
);
CREATE INDEX idx_versions_prompt ON versions(prompt_id, seq);

CREATE TABLE runs (
  id TEXT PRIMARY KEY, prompt_id TEXT, benchmark_id TEXT,
  provider TEXT, model TEXT, params_json TEXT,
  latency_ms INTEGER, tokens_in INTEGER, tokens_out INTEGER,
  cost REAL, error TEXT, response_path TEXT,  -- respuesta en .pes/runs/ (JSON)
  created_at TEXT
);
CREATE TABLE benchmarks (
  id TEXT PRIMARY KEY, prompt_id TEXT, config_json TEXT,
  summary_json TEXT, created_at TEXT
);

CREATE TABLE validation_cache (
  content_hash TEXT PRIMARY KEY, report_json TEXT, created_at TEXT
);
```

Invariantes:
- El índice se reconstruye por completo desde el workspace (`pes reindex`); corrupción
  del `.db` nunca implica pérdida de datos.
- Re-indexado incremental por `mtime + content_hash`.
- `versions` y `runs` sí tienen su fuente en `.pes/history/` y `.pes/runs/` (archivos);
  la tabla es solo índice de consulta.

## 6.4 Migraciones

- **Archivos**: campo `pes: N` en front-matter; migradores `N→N+1` idempotentes que
  corren al abrir el workspace (con backup previo automático).
- **Índice SQLite**: `PRAGMA user_version`; ante cambio de esquema se permite
  reconstrucción total (barato, es desechable).
- **Settings TOML**: campo `schema` con la misma política que archivos.
