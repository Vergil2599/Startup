# 5. Estructura de carpetas

## 5.1 Monorepo del proyecto

```
prompt-engineering-studio/
├── cmd/
│   └── pes/                      # main del binario único (GUI + CLI por subcomandos)
│       └── main.go
├── internal/                     # código no importable desde fuera (Go)
│   ├── domain/                   # ── ANILLO 1: entidades puras ──
│   │   ├── prompt.go             # Prompt, Block, BlockType
│   │   ├── template.go           # Template, herencia
│   │   ├── variable.go           # Variable, VariableScope
│   │   ├── version.go            # Version, Diff
│   │   ├── library.go            # Folder, Tag, Project
│   │   ├── run.go                # RunResult, BenchmarkRun
│   │   ├── validation.go         # ValidationReport, Finding, Severity
│   │   └── errors.go             # errores de dominio tipados
│   ├── core/                     # ── ANILLO 2: casos de uso ──
│   │   ├── promptsvc/
│   │   ├── composersvc/
│   │   ├── historysvc/
│   │   ├── librarysvc/
│   │   ├── variablesvc/
│   │   ├── templatesvc/
│   │   ├── runsvc/               # simulador + benchmark
│   │   └── ports.go              # TODAS las interfaces (Repository, LLMProvider…)
│   ├── template/                 # Template Engine (parser, render, herencia)
│   ├── validation/               # Validation Engine (pipeline + reglas builtin)
│   │   └── rules/
│   ├── storage/                  # ── ANILLO 3: infraestructura ──
│   │   ├── fsrepo/               # repositorio sobre archivos MD+YAML
│   │   ├── sqlindex/             # índice SQLite FTS5 + migraciones
│   │   ├── watcher/              # fsnotify → eventos
│   │   └── backup/               # backups rotados, cifrado age opcional
│   ├── export/                   # codecs: md, json, yaml, txt, html, pdf, clipboard
│   ├── importer/                 # detección de formato + mapeo a bloques
│   ├── plugin/
│   │   ├── host/                 # ciclo de vida, registro de extensiones
│   │   ├── wasmrt/               # runtime wazero + ABI
│   │   └── rpcrt/                # plugins por JSON-RPC/stdio
│   ├── ai/                       # cliente gRPC del sidecar + providers Go nativos
│   │   └── providers/            # ollama, openai-compat, lmstudio…
│   ├── settings/                 # config TOML en capas + migraciones
│   ├── events/                   # event bus tipado
│   └── app/                      # composición (DI manual): wire-up de todo
├── ui/                           # capa de vista (Wails)
│   ├── src/
│   │   ├── views/                # Builder, Library, Composer, Diff, Runner, Settings
│   │   ├── components/           # BlockCard, ScoreBadge, VariableChip…
│   │   ├── stores/               # estado de vista (NO lógica de negocio)
│   │   └── bindings/             # generados por Wails
│   └── wails.json
├── sidecar/                      # pes-ai (Python)
│   ├── pes_ai/
│   │   ├── server.py             # gRPC server (UDS/named pipe)
│   │   ├── optimizer/            # pipeline de optimización
│   │   ├── providers/            # adaptadores LLM async
│   │   ├── evaluate/             # consistencia, rúbricas, LLM-judge
│   │   └── generated/            # stubs protobuf
│   ├── pyproject.toml
│   └── tests/
├── proto/                        # contratos gRPC compartidos (fuente única)
│   ├── ai.proto
│   └── plugin.proto              # también ABI JSON-RPC documentada aquí
├── pkg/                          # ÚNICO código Go público
│   └── pluginsdk/                # SDK para autores de plugins (tipos + helpers)
├── plugins/                      # plugins de ejemplo / first-party
│   ├── example-exporter-wasm/
│   └── example-provider-python/
├── assets/                       # iconos, temas, plantillas builtin
│   └── templates/                # plantillas por categoría (software, devops, godot…)
├── docs/                         # esta documentación + ADRs + guías
│   └── adr/
├── scripts/                      # build, release, codegen
├── Taskfile.yml
├── go.mod
└── .github/workflows/            # CI: lint, test, build matrix, release
```

Reglas de dependencia verificadas en CI (lint de imports):
- `domain` no importa nada del proyecto.
- `core` importa solo `domain` (+ stdlib).
- `template`, `validation` importan solo `domain`.
- `storage`, `export`, `importer`, `plugin`, `ai` implementan puertos de `core/ports.go`.
- `ui/` y `cmd/` solo hablan con `core` a través de `app`.

## 5.2 Workspace del usuario (datos)

```
MiWorkspace/                      # cualquier carpeta elegida por el usuario (git-friendly)
├── prompts/
│   ├── desarrollo/
│   │   └── revisor-de-codigo.md  # front-matter YAML + bloques en Markdown
│   └── godot/…
├── templates/
│   └── software-development/base-coding.md
├── variables/
│   ├── global.yaml
│   └── proyecto-x.yaml
├── compositions/
│   └── prompt-final-godot.yaml   # capas + estrategia de merge
└── .pes/                         # metadatos de la app (regenerable salvo history)
    ├── index.db                  # SQLite FTS5 — DESECHABLE, se reconstruye
    ├── history/                  # snapshots de versiones (content-addressed)
    ├── runs/                     # resultados de simulador/benchmark (JSON)
    ├── backups/
    └── workspace.toml            # settings de proyecto
```

Configuración global de la app (fuera del workspace):
`~/.config/pes/` (Linux) · `~/Library/Application Support/pes/` (macOS) ·
`%APPDATA%\pes\` (Windows) → `settings.toml`, `plugins/`, `keyring-fallback.age`.
