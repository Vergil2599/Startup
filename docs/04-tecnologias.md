# 4. Tecnologías recomendadas y justificación

Restricción de partida: **Go y Python** como lenguajes del proyecto. Reparto:
Go para todo lo que debe ser rápido, portable y distribuible como binario único
(core, storage, validación, UI shell, CLI); Python para lo que se beneficia del
ecosistema de IA (optimizer, evaluación, adaptadores complejos).

## 4.1 Núcleo y aplicación de escritorio (Go)

| Componente | Elección | Justificación | Alternativas descartadas |
|---|---|---|---|
| Lenguaje del core | **Go 1.23+** | Binario único multiplataforma, concurrencia nativa (indexación, benchmark), arranque instantáneo, tipado fuerte | — (requisito) |
| Shell de escritorio | **Wails v2** | Ventana nativa ligera (WebView del SO, sin bundle Chromium), bindings Go↔vista generados, ~10–20 MB vs. >150 MB de Electron | **Electron** (peso, RAM, requeriría Node); **Fyne/Gio** (widgets insuficientes para un IDE: paneles acoplables, editor rico, drag&drop complejo) |
| Capa de vista | **Vue 3 + TypeScript (capa fina)** dentro de Wails | Un IDE necesita DOM: paneles acoplables, DnD, virtual scrolling. La vista no contiene lógica de negocio (toda en Go vía bindings) | **templ + HTMX (100 % Go)**: viable y se mantiene como plan B documentado, pero el round-trip por binding para cada interacción de editor degrada la UX de un editor rico |
| Editor de texto de bloques | **CodeMirror 6** embebido | Resaltado de `{{variables}}`, folding, undo/redo ilimitado, rendimiento con documentos grandes | Monaco (pesado), textarea (insuficiente) |
| CLI | **cobra** | Estándar de facto, subcomandos, autocompletado | urfave/cli (equivalente; cobra tiene más tracción) |
| Índice/búsqueda | **SQLite + FTS5** (driver `modernc.org/sqlite`, CGO-free) | Búsqueda full-text < 50 ms sobre miles de docs, un solo archivo, cero servicios; driver puro Go mantiene cross-compilación trivial | Bleve (más RAM, menos maduro); mattn/go-sqlite3 (CGO complica builds) |
| Watcher FS | **fsnotify** | Detección de ediciones externas (git, Obsidian) | polling (costoso) |
| Runtime de plugins WASM | **wazero** | Runtime WASM puro Go, sin CGO, sandbox por capacidades | wasmtime-go (CGO); `plugin` stdlib (no portable, no sandbox) |
| RPC con sidecar/plugins | **gRPC sobre Unix socket / named pipe** (sidecar) y **JSON-RPC 2.0 / stdio** (plugins externos) | gRPC: contratos versionados con protobuf, streaming para tokens; JSON-RPC/stdio: cero dependencias para autores de plugins | REST local (sin streaming tipado), puertos TCP (superficie de ataque) |
| Diff de versiones | **go-diff** (Myers) + diff estructural propio por bloques | El comparador necesita diff semántico (bloques añadidos/eliminados), no solo texto | solo diff textual |
| Config | **TOML** (`BurntSushi/toml`) | Legible, comentable, estándar en herramientas dev | JSON (sin comentarios), YAML para config (errores por indentación) |
| Cifrado opcional | **age** (filippo.io/age, X25519+XChaCha20) | Auditado, simple, passphrase o clave | GPG (UX hostil) |
| Credenciales | **keyring del SO** (zalando/go-keyring) + fallback archivo age | No guardar API keys en texto plano | — |
| PDF export | **HTML → PDF** con chromedp si hay Chrome, fallback puro Go (go-pdf/fpdf) para layout simple | PDF fiel al HTML exportado sin dependencia dura | wkhtmltopdf (abandonado) |
| Logging | **log/slog** (stdlib) | Estructurado, sin dependencia | zap/zerolog (innecesarios aquí) |
| Testing | **stdlib testing + testify + rapid** (property-based para template/merge) | Ver [10-plan-pruebas.md](10-plan-pruebas.md) | — |

## 4.2 Sidecar de IA (Python)

| Componente | Elección | Justificación |
|---|---|---|
| Runtime | **Python 3.12+**, empaquetado con **uv** | Resolución rápida y reproducible; instalable como extra opcional |
| Servidor | **grpcio** + protobuf compartidos con Go (un solo `proto/`) | Contrato único generado para ambos lados |
| Adaptadores LLM | **httpx** (async) contra APIs OpenAI-compatible, Ollama, LM Studio, OpenRouter; SDKs oficiales solo si aportan (anthropic) | Mínimas dependencias; streaming SSE |
| Optimizer | Pipeline propio: reglas + reescritura con LLM local; **sin frameworks pesados** (no LangChain) | LangChain añade acoplamiento y churn; el dominio es acotado |
| Evaluación/consistencia | **rapidfuzz** (similitud), rúbricas LLM-judge opcionales | Métrica de consistencia del benchmark |
| Distribución | Paquete `pes-ai` en PyPI + binarios congelados (PyInstaller) para usuarios sin Python | El usuario medio no debe pelear con entornos |

**Regla dura**: el sidecar nunca es requisito. Toda llamada pasa por el puerto Go
`Optimizer`/`LLMProvider`; si el sidecar no responde en el handshake, la app marca las
features de IA como no disponibles y todo lo demás funciona.

## 4.3 Contratos y build

- **protobuf** en `proto/` genera Go (`protoc-gen-go`) y Python (`grpcio-tools`) — fuente única.
- **Task** (Taskfile) como runner de build multiplataforma; CI en GitHub Actions
  (matriz linux/macos/windows).
- **golangci-lint** + **ruff/mypy** para calidad; **gofumpt** formateo.
- Versionado semántico; la **API de plugins** tiene su propio semver independiente.

## 4.4 Matriz de riesgo tecnológico (resumen; detalle en 09)

| Elección | Riesgo principal | Mitigación |
|---|---|---|
| Wails v2 | WebView inconsistente entre SOs | Suite E2E por plataforma; features CSS conservadoras |
| Vista Vue/TS | "Lógica que se filtra" a la vista | Regla de revisión: la vista no importa nada que no sean bindings + componentes; CLI cubre el 100 % de casos de uso como prueba |
| Sidecar Python | Fricción de instalación | Binarios PyInstaller + detección/instalación guiada; todo opcional |
| wazero/WASM | Ecosistema de plugins joven | API de plugins también disponible vía JSON-RPC/stdio (cualquier lenguaje) |
