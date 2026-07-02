# 7. Sistema de plugins

## 7.1 Objetivos

- Que el núcleo permanezca pequeño: proveedores LLM, exportadores, reglas de validación,
  tipos de bloque e integraciones (Obsidian, MCP, GitHub…) se implementan como plugins.
- **Seguridad por defecto**: un plugin no puede tocar disco ni red sin capacidades
  concedidas explícitamente por el usuario.
- **Cualquier lenguaje**: primera clase para Go→WASM y Python; posible en cualquier
  lenguaje que hable JSON-RPC por stdio o compile a WASM.
- **API estable versionada** (semver propio, independiente del de la app).

## 7.2 Dos runtimes, un solo modelo de extensión

| Runtime | Para qué | Sandbox | Latencia |
|---|---|---|---|
| **WASM (wazero)** | Exportadores, importadores, reglas de validación, filtros de template, tipos de bloque | Total (sin FS/red salvo capacidades WASI concedidas) | µs, in-process |
| **Proceso externo (JSON-RPC 2.0 / stdio)** | Proveedores LLM, integraciones (Obsidian, Git, MCP), plugins Python | Proceso separado, permisos del SO + declaración de capacidades | ms, aceptable para I/O |

Ambos runtimes exponen **el mismo catálogo de puntos de extensión**; el host normaliza.

## 7.3 Puntos de extensión (v1 de la API)

| Extension point | Contrato (resumen) |
|---|---|
| `llm.provider` | `capabilities()`, `complete(request) → stream(chunks) \| response` |
| `export.codec` | `formats() → [{id, ext, mime}]`, `export(promptIR) → bytes` |
| `import.codec` | `sniff(bytes) → confidence`, `import(bytes) → promptIR` |
| `validation.rule` | `meta() → {id, severity_default, category}`, `check(promptIR) → findings[]` |
| `block.type` | `descriptor() → {id, label, icon, placeholder, validation_hints}` |
| `template.filter` | `apply(value, args) → value` (p. ej. `{{x\|kebab}}`) |
| `command` | comando para la paleta: `descriptor()`, `run(contextIR) → actions[]` |
| `panel` (v2) | panel de UI **declarativo** (JSON schema de widgets); nunca HTML/JS arbitrario |

`promptIR` es la representación intermedia JSON del prompt (estable, versionada) —
los plugins nunca ven structs internos de Go.

## 7.4 Manifiesto (`plugin.toml`)

```toml
[plugin]
id = "dev.ejemplo.obsidian-sync"
name = "Obsidian Sync"
version = "0.3.1"
api = "1.x"                        # rango de API del host soportado
runtime = "process"                # "wasm" | "process"
entry = "bin/obsidian-sync"        # o "plugin.wasm"
sdk_lang = "python"

[capabilities]                     # todo denegado por defecto
fs_read  = ["${workspace}"]        # rutas parametrizadas, sin comodines fuera
fs_write = ["${workspace}/prompts"]
network  = ["127.0.0.1"]           # hosts permitidos; vacío = sin red
secrets  = []                      # nombres de credenciales que puede pedir

[contributes]
extension_points = ["import.codec", "command"]
```

Flujo de instalación: descubrimiento en `~/.config/pes/plugins/<id>/` →
validación de manifiesto y compatibilidad `api` → **pantalla de consentimiento**
mostrando capacidades → carga → registro de contribuciones. (Ciclo de vida
completo diagramado en [03-diagramas.md §3.6](03-diagramas.md).)

## 7.5 Aislamiento y robustez

- **WASM**: memoria propia, sin WASI salvo lo concedido; límite de memoria y de tiempo
  de CPU por invocación (context deadline); pánico del plugin ≠ pánico del host.
- **Proceso**: lanzado bajo demanda, `stdin/stdout` JSON-RPC, `stderr` → log del plugin;
  timeout y circuit breaker (3 fallos → estado *Suspendido*, reactivación manual).
- Un plugin suspendido degrada solo su funcionalidad; el resto de la app no se entera.
- Las llamadas del host al plugin son siempre con datos serializados (IR), nunca
  referencias compartidas.

## 7.6 SDKs para autores

- **`pkg/pluginsdk` (Go)**: tipos del IR, helpers de registro, target `GOOS=wasip1 GOARCH=wasm`.
- **`pes-plugin-sdk` (Python, en PyPI)**: clase base por extension point; el autor
  implementa métodos y el SDK gestiona el bucle JSON-RPC. Un provider LLM mínimo ≈ 30 líneas.
- Plantillas `pes plugin new --lang go|python` en la CLI.
- Los plugins de ejemplo en `plugins/` del monorepo sirven como tests de contrato vivos.

## 7.7 Versionado y compatibilidad

- La API de plugins usa semver propio (`pes-plugin-api 1.x`); el host publica qué rango
  soporta y rechaza (con mensaje claro) plugins fuera de rango.
- Cambios breaking → bump mayor + guía de migración + periodo de soporte dual (host
  soporta `1.x` y `2.x` durante al menos una versión menor de la app).
- Suite de **tests de contrato** ejecutable por autores: `pes plugin verify ./mi-plugin`.

## 7.8 Distribución (fases)

- **v1**: instalación manual (carpeta o `pes plugin install ./ruta|URL de release git`).
- **v2**: registro/marketplace con firmas (minisign) y verificación de integridad;
  también para paquetes de plantillas.
