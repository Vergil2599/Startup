# 2. Arquitectura completa

## 2.1 Vista general

PES sigue **Clean Architecture** con separación estricta en anillos. La regla de
dependencia es unidireccional: nada del dominio conoce infraestructura, UI ni frameworks.

```
┌────────────────────────────────────────────────────────────┐
│  Presentación (UI Wails / CLI / API local)                  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Aplicación (casos de uso, orquestación, DTOs)        │  │
│  │  ┌────────────────────────────────────────────────┐  │  │
│  │  │  Dominio (entidades, value objects, reglas)     │  │  │
│  │  └────────────────────────────────────────────────┘  │  │
│  │  Puertos (interfaces): Storage, LLM, Export, Plugin  │  │
│  └──────────────────────────────────────────────────────┘  │
│  Infraestructura (SQLite, FS, gRPC→Python, WASM, HTTP)     │
└────────────────────────────────────────────────────────────┘
```

## 2.2 Procesos del sistema

PES se compone de **tres procesos**, dos de ellos opcionales:

| Proceso | Lenguaje | Obligatorio | Rol |
|---|---|---|---|
| `pes` (app de escritorio) | Go (Wails v2) | Sí | UI + Core Engine + Storage + Plugins WASM |
| `pes-ai` (sidecar IA) | Python | **No** | Optimizer, Benchmark avanzado, adaptadores LLM |
| Plugins externos | Cualquiera | No | Procesos JSON-RPC/stdio lanzados bajo demanda |

- El binario Go es **autosuficiente**: builder, biblioteca, validador, historial,
  import/export, plantillas y variables funcionan sin `pes-ai` y sin red.
- `pes-ai` se comunica por **gRPC sobre Unix socket / named pipe** (nunca puerto TCP
  expuesto). Si no está instalado o está desactivado en Settings, la UI oculta/deshabilita
  las funciones de IA con un estado claro ("modo offline puro").
- También existe `pes` como **CLI** (mismo binario, subcomandos: `pes export`, `pes validate`,
  `pes render`, `pes bench`) para integración en CI y con otras herramientas.

## 2.3 Módulos del Core (Go)

Cada módulo es un paquete Go independiente con interfaz pública propia, inyectado por
constructor (DI manual, sin framework mágico). Bajo acoplamiento: los módulos se comunican
solo a través de interfaces del dominio y de un **event bus** interno.

### 2.3.1 Domain (`internal/domain`)
Entidades puras, sin dependencias externas:
`Prompt`, `Block`, `BlockType`, `Template`, `Variable`, `Project`, `Version`,
`Tag`, `Folder`, `ValidationReport`, `RunResult`, `BenchmarkRun`, `Composition`.
Contiene las reglas invariantes (p. ej. "un prompt renderizado no puede contener
variables sin resolver si `strict=true`").

### 2.3.2 Core Engine (`internal/core`)
Casos de uso (application services). Ejemplos de contratos:

```go
type PromptService interface {
    Create(ctx, CreatePromptCmd) (Prompt, error)
    UpdateBlocks(ctx, id ID, blocks []Block) (Prompt, error)
    Render(ctx, id ID, scope VariableScope, opts RenderOpts) (string, error)
    Duplicate(ctx, id ID) (Prompt, error)
}

type ComposerService interface {
    Compose(ctx, layers []PromptRef, strategy MergeStrategy) (Prompt, error)
}

type HistoryService interface {
    Snapshot(ctx, promptID ID, comment string) (Version, error)
    Diff(ctx, a, b VersionRef) (Diff, error)
    Restore(ctx, promptID ID, v VersionRef) error
}
```

### 2.3.3 Template Engine (`internal/template`)
- Sintaxis `{{variable}}` con filtros (`{{language|upper}}`), condicionales por bloque
  y **herencia de plantillas** (una plantilla declara `extends: base-id` y
  sobrescribe/añade bloques).
- Resolución de variables por ámbito: `bloque > prompt > proyecto > global > default`.
- Motor propio minimalista (no `text/template` expuesto al usuario, para mantener
  la sintaxis simple, segura y validable).

### 2.3.4 Validation Engine (`internal/validation`)
Pipeline de reglas deterministas (sin IA), cada regla es un `Rule` registrable
(también por plugins):

- `MissingSectionRule` (falta Objective/Output…) — severidad configurable
- `UndefinedVariableRule`, `EmptyBlockRule`, `DuplicateRuleRule`
- `ContradictionRule` (heurísticas léxicas: "nunca X" + "siempre X")
- `AmbiguityRule` (objetivos con verbos vagos, métricas ausentes)
- `ConflictingConstraintRule`, `ClarityRule` (longitud de frases, voz pasiva…)

Salida: `ValidationReport` con findings (regla, severidad, bloque, sugerencia) y
**puntuación 0–100** ponderada y explicable (desglose por categoría).

### 2.3.5 Storage (`internal/storage`)
Estrategia dual (detalle en [13-persistencia.md](13-persistencia.md)):
- **Fuente de verdad**: archivos Markdown con front-matter YAML en el workspace del usuario.
- **Índice**: SQLite (FTS5) reconstruible en cualquier momento; nunca es fuente de verdad.
- Puerto único `Repository` + `Indexer`; watcher de FS para cambios externos (git pull, Obsidian).

### 2.3.6 Plugin System (`internal/plugin`)
Dos runtimes (detalle en [07-sistema-plugins.md](07-sistema-plugins.md)):
- **WASM (wazero)**: sandboxeado, sin permisos por defecto, capacidades declaradas en manifiesto.
- **Proceso externo (JSON-RPC 2.0 / stdio)**: para Python u otros lenguajes.
Puntos de extensión: proveedores LLM, exportadores, importadores, reglas de validación,
tipos de bloque, comandos de paleta, paneles de UI (declarativos).

### 2.3.7 AI Engine (puerto en Go, implementación en Python)
El Go define el puerto:

```go
type LLMProvider interface {
    Name() string
    Capabilities() Caps            // streaming, tokens, costo…
    Complete(ctx, Request) (Response, error)
}
type Optimizer interface {
    Optimize(ctx, prompt Prompt, goals []Goal) (Suggestions, error)
}
```

Implementaciones: nativas en Go para APIs simples (OpenAI-compatible, Ollama HTTP) y
delegadas al sidecar Python para lo complejo (optimización, evaluación de calidad,
proveedores exóticos). **Todo detrás del mismo puerto**: el core no sabe quién responde.

### 2.3.8 Export / Import Engine (`internal/export`, `internal/import`)
Registro de codecs: `md`, `json`, `yaml`, `txt`, `html`, `pdf` (vía HTML→PDF embebido),
`clipboard`. Importadores con detección de formato y mapeo asistido de secciones →
bloques. Ambos extensibles por plugins.

### 2.3.9 Testing Engine — Simulador y Benchmark (`internal/run`)
- **Simulador**: ejecuta un prompt renderizado contra 1..N proveedores, guarda
  `RunResult` (respuesta, latencia, tokens, costo estimado, parámetros).
- **Benchmark**: matriz prompt × modelos × repeticiones; agrega métricas
  (tiempo, tokens, costo, consistencia por similitud, calidad opcional por rúbrica/LLM-judge
  en el sidecar). Ejecución concurrente con límites y cancelación.

### 2.3.10 Settings (`internal/settings`)
Configuración en capas (defaults → global → proyecto), archivo TOML legible,
API tipada, migraciones de esquema de settings.

### 2.3.11 Event Bus (`internal/events`)
Pub/sub interno tipado (`PromptSaved`, `ValidationFinished`, `RunCompleted`…).
Es el mecanismo por el que UI, indexador, autoguardado e historial reaccionan
sin acoplarse a los servicios.

## 2.4 Frontend (capa de presentación)

- **Wails v2**: la ventana nativa carga una vista web; **toda la lógica vive en Go**,
  expuesta a la vista mediante bindings generados. La vista es deliberadamente "tonta":
  render + eventos.
- La UI consume exclusivamente la **misma API de casos de uso** que la CLI — garantía
  de que ninguna lógica se cuela en la presentación.
- Vista construida con componentes web ligeros (detalle y justificación en
  [04-tecnologias.md](04-tecnologias.md); la alternativa 100 % Go con templ+HTMX se
  evalúa allí).

## 2.5 Flujo de datos típico (guardar y validar un prompt)

1. UI emite `UpdateBlocks` → binding Wails → `PromptService`.
2. `PromptService` valida invariantes de dominio, persiste vía `Repository`
   (escribe el archivo .md), publica `PromptSaved`.
3. Suscriptores: `Indexer` actualiza SQLite/FTS; `HistoryService` crea snapshot si
   procede (política de autosnapshot); `ValidationEngine` re-valida en background y
   publica `ValidationFinished`.
4. UI recibe el evento (push por el bridge de Wails) y actualiza la puntuación en vivo.

Ningún paso requiere red. Si el Optimizer está activo, es un paso *adicional* y
explícito del usuario.

## 2.6 Concurrencia y rendimiento

- Indexación y validación en goroutines con colas y *debounce*; la UI nunca bloquea.
- Búsqueda: FTS5 + caché LRU de resultados; carga diferida de cuerpos (la lista usa
  solo metadatos del índice).
- Benchmark: worker pool con límite configurable por proveedor y `context.Context`
  para cancelación total.

## 2.7 Seguridad

- Sin telemetría; cualquier feature de red futura será opt-in explícito.
- Sidecar y plugins externos solo por socket local/stdio; nada escucha en TCP.
- Plugins WASM sin FS/red salvo capacidades concedidas por el usuario.
- Cifrado opcional del workspace (age / XChaCha20-Poly1305) y de credenciales de
  proveedores (keyring del SO cuando exista; archivo cifrado como fallback).
- Backups automáticos rotados del workspace (detalle en persistencia).

## 2.8 Decisiones de arquitectura registradas (ADR)

Las decisiones se registran como ADRs en `docs/adr/`. Las iniciales:

| ADR | Decisión | Alternativa descartada | Motivo |
|---|---|---|---|
| 001 | Wails v2 para escritorio | Electron; Fyne/Gio | Peso/RAM vs. madurez de widgets complejos |
| 002 | Archivos MD+YAML como fuente de verdad | Solo SQLite | Git-friendly, Obsidian, durabilidad, transparencia |
| 003 | SQLite FTS5 como índice desechable | Bleve | FTS5 más probado, un solo store auxiliar |
| 004 | Sidecar Python por gRPC/UDS | Embeber CPython; solo Go | Ecosistema IA de Python sin sacrificar binario Go |
| 005 | Plugins WASM (wazero) + JSON-RPC | `plugin` nativo de Go | El paquete `plugin` no es portable ni sandboxeable |
| 006 | DI manual por constructores | wire/fx | Simplicidad, trazabilidad, sin magia |
