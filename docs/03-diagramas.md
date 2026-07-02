# 3. Diagramas

Todos los diagramas están en Mermaid (renderizables en GitHub, Obsidian y VS Code).

## 3.1 C4 — Nivel 1: Contexto

```mermaid
graph TB
    U[Usuario<br/>Prompt/AI Engineer, Dev, Investigador]
    PES[Prompt Engineering Studio<br/>App de escritorio offline-first]
    LLML[LLMs locales<br/>Ollama · llama.cpp · LM Studio]
    LLMR[LLMs remotos opcionales<br/>OpenRouter · APIs OpenAI-compat]
    GIT[Git / GitHub<br/>workspace versionado]
    OBS[Obsidian / editores externos<br/>mismos archivos .md]
    TOOLS[Herramientas de agentes<br/>Claude Code · Cursor · Gemini CLI · MCP · OpenClaw]

    U -->|crea, valida, prueba prompts| PES
    PES -.->|simulador / benchmark / optimizer opcional| LLML
    PES -.->|solo si el usuario lo habilita| LLMR
    PES <-->|archivos MD+YAML| GIT
    PES <-->|workspace compartido| OBS
    PES -->|export / CLI / integraciones| TOOLS
```

## 3.2 C4 — Nivel 2: Contenedores

```mermaid
graph TB
    subgraph Escritorio["Máquina del usuario"]
        subgraph APP["pes (binario Go, Wails v2)"]
            UI[UI WebView<br/>capa de vista fina]
            BIND[Bindings Wails]
            CORE[Core Engine<br/>casos de uso]
            DOM[Dominio]
            VAL[Validation Engine]
            TPL[Template Engine]
            EXP[Export/Import Engine]
            RUN[Testing Engine<br/>Simulador · Benchmark]
            PLG[Plugin Host<br/>WASM wazero]
            STG[Storage<br/>FS + índice SQLite]
            BUS[(Event Bus)]
        end
        CLI[pes CLI<br/>mismo binario]
        AI[pes-ai · sidecar Python<br/>Optimizer · adaptadores LLM<br/>OPCIONAL]
        EXT[Plugins externos<br/>JSON-RPC/stdio]
        WS[(Workspace<br/>.md + .pes/)]
        DB[(SQLite FTS5<br/>índice desechable)]
    end

    UI --> BIND --> CORE
    CLI --> CORE
    CORE --> DOM
    CORE --> VAL & TPL & EXP & RUN
    CORE --> STG
    STG --> WS
    STG --> DB
    CORE --> BUS
    PLG --- CORE
    EXT --- PLG
    RUN -.->|gRPC / UDS| AI
    CORE -.->|gRPC / UDS| AI
```

## 3.3 C4 — Nivel 3: Componentes del Core Engine

```mermaid
graph LR
    subgraph Core["internal/core"]
        PS[PromptService]
        CS[ComposerService]
        HS[HistoryService]
        LS[LibraryService<br/>carpetas·tags·búsqueda]
        VS[VariableService]
        TS[TemplateService]
        RS[RunService]
        SS[SettingsService]
    end
    subgraph Puertos["Puertos (interfaces)"]
        REPO[Repository]
        IDX[Indexer/Search]
        LLM[LLMProvider]
        OPT[Optimizer]
        CODEC[Exporter/Importer]
        RULE[ValidationRule]
    end
    PS --> REPO & RULE
    LS --> IDX
    TS --> REPO
    CS --> PS
    HS --> REPO
    RS --> LLM
    RS -.-> OPT
    PS --> CODEC
```

## 3.4 Secuencia: edición con validación en vivo

```mermaid
sequenceDiagram
    participant UI
    participant PS as PromptService
    participant FS as Storage(FS)
    participant BUS as EventBus
    participant IDX as Indexer
    participant VAL as ValidationEngine

    UI->>PS: UpdateBlocks(id, blocks)
    PS->>FS: write prompt.md (atómico)
    PS->>BUS: publish PromptSaved
    PS-->>UI: Prompt actualizado
    par en background
        BUS->>IDX: PromptSaved
        IDX->>IDX: reindexar FTS5
    and
        BUS->>VAL: PromptSaved (debounce 300ms)
        VAL->>VAL: pipeline de reglas
        VAL->>BUS: publish ValidationFinished(report)
        BUS-->>UI: push score + findings
    end
```

## 3.5 Secuencia: benchmark multi-modelo

```mermaid
sequenceDiagram
    participant UI
    participant RS as RunService
    participant P1 as Provider Ollama (Go)
    participant P2 as Provider OpenAI-compat (Go)
    participant AI as pes-ai (Python, opcional)

    UI->>RS: Benchmark(prompt, [ollama:llama3, openrouter:x], reps=3)
    RS->>RS: render prompt (variables resueltas)
    par workers concurrentes
        RS->>P1: Complete(req) ×3
        RS->>P2: Complete(req) ×3
    end
    P1-->>RS: respuestas + latencia + tokens
    P2-->>RS: respuestas + latencia + tokens + costo
    opt calidad con LLM-judge habilitado
        RS->>AI: Evaluate(respuestas, rúbrica)
        AI-->>RS: puntuaciones
    end
    RS->>RS: agregar métricas, persistir BenchmarkRun
    RS-->>UI: tabla comparativa
```

## 3.6 Ciclo de vida de un plugin

```mermaid
stateDiagram-v2
    [*] --> Descubierto: escaneo de ~/.pes/plugins
    Descubierto --> Verificado: manifiesto válido +<br/>API version compatible
    Verificado --> Consentido: usuario aprueba capacidades
    Consentido --> Cargado: WASM instanciado /<br/>proceso lanzado
    Cargado --> Activo: registra extensiones<br/>(providers, codecs, reglas…)
    Activo --> Suspendido: error/timeout → aislar
    Suspendido --> Activo: reintento manual
    Activo --> Descargado: usuario desactiva
    Descargado --> [*]
```

## 3.7 Modelo Entidad-Relación (resumen; detalle en 06)

```mermaid
erDiagram
    PROJECT ||--o{ FOLDER : contiene
    FOLDER ||--o{ PROMPT : contiene
    PROMPT ||--|{ BLOCK : "compuesto por"
    PROMPT ||--o{ VERSION : historial
    PROMPT }o--o{ TAG : etiquetado
    TEMPLATE ||--o{ PROMPT : instancia
    TEMPLATE |o--o| TEMPLATE : extiende
    PROJECT ||--o{ VARIABLE : "ámbito proyecto"
    PROMPT ||--o{ RUNRESULT : ejecuciones
    BENCHMARK ||--|{ RUNRESULT : agrupa
    COMPOSITION }o--|{ PROMPT : "capas ordenadas"
```

## 3.8 Flujo de datos de persistencia

```mermaid
graph LR
    EDIT[Edición en UI] -->|escritura atómica| MD[Workspace<br/>*.md + front-matter YAML]
    EXTERNO[git pull / Obsidian /<br/>edición externa] --> MD
    MD -->|fsnotify| WATCH[Watcher]
    WATCH --> IDX[Indexer]
    IDX --> SQL[(SQLite FTS5<br/>reconstruible)]
    SQL --> SEARCH[Búsqueda instantánea]
    MD --> SNAP[Snapshots<br/>.pes/history/]
    MD --> BAK[Backups rotados<br/>opcionalmente cifrados]
```
