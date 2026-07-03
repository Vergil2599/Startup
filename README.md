# Prompt Engineering Studio (PES) — v1.0

El IDE offline-first para Prompt Engineering: crear, componer, validar, optimizar,
probar, versionar y reutilizar prompts para cualquier LLM, local o remoto.
*Lo que VS Code es para el código, PES lo es para los prompts.*

## Estado: v1 lista para probar

➡️ **[Guía de instalación paso a paso con todas las dependencias → INSTALL.md](INSTALL.md)**

```bash
go build -o pes ./cmd/pes

./pes init -w ~/mis-prompts        # workspace + 4 plantillas de inicio
./pes ui   -w ~/mis-prompts        # → abre http://127.0.0.1:8787 en tu navegador
```

La UI web local incluye: **biblioteca** con búsqueda instantánea, **builder por
bloques** (activar/desactivar, reordenar, añadir), **validación en vivo** con
puntuación 0–100 explicable, **vista previa** renderizada con variables,
**historial** de versiones con restauración, **runner** multi-proveedor,
**comparador** (diff estructural + texto), **composer** de capas, exportación
(md/txt/json/yaml/html/pdf/portapapeles), temas claro/oscuro y atajos
(`/` buscar, `Ctrl+N` nuevo, `Ctrl+S` versión).

### CLI completa (misma lógica que la UI)

| Comando | Qué hace |
|---|---|
| `init` · `new` · `list` · `search` | Workspace, creación (con `-t plantilla`), biblioteca, búsqueda FTS |
| `render` · `validate --min-score N` | Render con variables; validación con gate para CI |
| `snapshot` · `history` · `restore` | Versionado content-addressed; la historia nunca se reescribe |
| `diff` · `compose` | Comparador estructural+texto; composición por capas (concat/replace) |
| `run -P proveedor` | Ejecuta contra Ollama / LM Studio / APIs OpenAI-compatibles (`.pes/providers.yaml`) |
| `bench -n 5 -P a -P b` | Benchmark multi-modelo: p50/p95, tokens, fallos y **consistencia** entre repeticiones |
| `duplicate` · `vars [set\|rm] [-p proyecto]` | Copiar prompts; gestionar variables globales o de proyecto |
| `mcp` | **Servidor MCP**: expone tu biblioteca a Claude Code, Cursor y agentes (search/get/render/validate) |
| `lint --min-score 70 --format github` | Valida TODO el workspace; gate para CI con anotaciones de GitHub |
| `golden set\|check --threshold 0.6` | Respuesta de referencia por prompt y detección de regresiones (deriva) |
| `optimize` | Sugerencias del sidecar `pes-ai` (opcional, nunca modifica el prompt) |
| `export -f md\|txt\|json\|yaml\|html\|pdf` · `import` | Exportación e importación con round-trip |
| `backup --passphrase X --keep N` | Backups tar.gz rotados, cifrado age opcional |
| `plugin list\|enable\|disable` | Plugins JSON-RPC/stdio con consentimiento explícito |
| `ui --addr 127.0.0.1:8787` | Interfaz web local |

### Proveedores LLM (opcional — todo lo demás funciona sin red)

```yaml
# ~/mis-prompts/.pes/providers.yaml
providers:
  - {name: ollama-local, type: ollama, base_url: "http://localhost:11434", model: llama3}
  - {name: lmstudio,     type: openai, base_url: "http://localhost:1234",  model: qwen2.5}
  - {name: openrouter,   type: openai, base_url: "https://openrouter.ai/api", model: meta-llama/llama-3.1-70b, api_key_env: OPENROUTER_KEY}
```

### IA opcional (`pes-ai`)

El Optimizer corre como sidecar Python (solo stdlib, sin pip install):
se autodetecta desde el árbol de fuentes, `pes-ai` en PATH o `PES_AI_CMD`.
Sin él, PES funciona al 100 % — la IA es una ayuda, nunca una dependencia.

### Plugins

Cualquier lenguaje que hable JSON-RPC 2.0 por stdio. Ejemplo completo en Python
en [`plugins/example-python/`](plugins/example-python/) (regla de validación +
proveedor LLM de eco). Se colocan en `.pes/plugins/<id>/` y requieren
`pes plugin enable <id>` (consentimiento explícito; capacidades declaradas en
`plugin.toml`; circuit breaker ante fallos).

## Arquitectura

- **Go**: core engine (Clean Architecture), storage, validación, CLI, servidor UI.
- **Python**: sidecar opcional de IA y plugins.
- **Datos**: archivos Markdown+YAML (git/Obsidian-friendly, fuente de verdad) +
  índice SQLite FTS5 desechable (`pes reindex --rebuild` lo reconstruye siempre).
- **Rendimiento verificado por tests**: 10 000 prompts → indexación ~6 s,
  búsqueda p95 ~22 ms, sync incremental ~174 ms.
- Sin telemetría; el servidor escucha solo en localhost.

➡️ [Documentación de diseño (15 entregables)](docs/README.md) · [Roadmap](docs/08-roadmap.md)

## Desarrollo

```bash
go test ./...                                  # suite Go completa (~170 tests, incluye presupuestos)
go test -race -short ./...                     # con detector de carreras
python3 -m unittest discover -s sidecar/tests  # tests del sidecar
python3 scripts/ui_journey.py                  # E2E de la UI con Chromium real (requiere `pes ui` en :8822)
```

La v1 pasó una ronda de **pruebas exploratorias simulando usuarios reales**
(novato, usuario de Obsidian con ediciones externas, power user, entradas
hostiles, concurrencia CLI+UI y una jornada completa en navegador real con
Playwright). Los 5 bugs encontrados —colisión de slugs con pérdida de datos,
import con ID duplicado, índice roto tras reemplazo externo de archivos,
errores de indexado anónimos y typos silenciosos en variables de proyecto—
están corregidos con tests de regresión en `internal/core/regression_test.go`.

Nota de implementación v1: la UI de escritorio se sirve como web local embebida
en el binario (`pes ui`); el shell Wails previsto en el diseño la envolverá en
v1.x sin cambios de lógica (la vista solo consume la API JSON). El transporte
del sidecar es JSON-RPC/stdio (el mismo de los plugins); la migración a gRPC/UDS
del ADR-004 queda para cuando haya streaming de tokens.
