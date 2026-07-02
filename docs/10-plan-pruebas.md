# 10. Plan de pruebas

## 10.1 Estrategia general

Pirámide clásica adaptada a Clean Architecture: la masa de tests vive en dominio y
casos de uso (rápidos, sin I/O); la infraestructura se prueba contra recursos reales
(SQLite en memoria/tmp, FS temporal); la UI se cubre con E2E finos sobre los flujos clave.

| Capa | Tipo | Herramientas | Objetivo de cobertura |
|---|---|---|---|
| Dominio | Unitarias puras | `testing` + testify | ≥ 90 % |
| Template/Validation/Composer | Unitarias + **property-based** | rapid | ≥ 90 %, invariantes |
| Casos de uso (core) | Unitarias con puertos falsos (fakes, no mocks frágiles) | testify | ≥ 85 % |
| Storage / índice | Integración (FS tmp + SQLite real) | testing + fixtures | flujos completos |
| Sidecar Python | Unitarias (pytest) + contrato gRPC | pytest, grpcio-testing | ≥ 85 % |
| Plugins | **Tests de contrato** del ABI | `pes plugin verify` | 100 % de extension points |
| CLI | Golden tests (salida esperada) | testscript | comandos públicos |
| UI | E2E de flujos críticos | Playwright contra build Wails dev | ~12 flujos |
| Rendimiento | Benchmarks con presupuesto | `go test -bench` + workspace sintético | sin regresión |

## 10.2 Áreas con tests específicos obligatorios

### Template Engine
- Property-based: `render(parse(x))` estable; variables sin resolver detectadas siempre
  en modo estricto; sin inyección (contenido de variable nunca se interpreta como sintaxis).
- Herencia: cadenas de 3+ niveles, detección de ciclos, políticas `inherit/override/append`.
- Resolución de ámbitos: precedencia bloque > prompt > proyecto > global.

### Validation Engine
- Cada regla: tabla de casos positivos/negativos + casos límite (bloques vacíos,
  solo desactivados, unicode).
- Puntuación: monotónica (añadir un finding nunca sube el score), determinista,
  desglose suma consistente.

### Storage
- Escritura atómica: kill del proceso simulado entre tmp y rename → archivo previo intacto.
- Round-trip: `parse(serialize(prompt)) == prompt` para todo el corpus de fixtures.
- Watcher: edición externa → reindexado; conflicto por hash → evento de conflicto (nunca sobrescritura silenciosa).
- Reconstrucción total del índice = mismo resultado que incremental (equivalencia).

### Historial / Comparador
- Snapshot → restore → contenido idéntico byte a byte.
- Diff estructural: bloques añadidos/eliminados/reordenados detectados correctamente
  (property-based con permutaciones aleatorias).

### Simulador / Benchmark
- Providers contra **servidores mock** (httptest) con latencias y errores inyectados;
  streaming interrumpido; cancelación por contexto limpia el pool.
- Agregación de métricas verificada con datos sintéticos conocidos.
- Smoke tests opcionales (tag `-tags=live`) contra Ollama real — no bloquean CI.

### Import/Export
- Round-trip por formato: `import(export(p)) ≡ p` para MD/JSON/YAML.
- Golden files para HTML/PDF/TXT (comparación estable, PDF por contenido de texto extraído).
- Importador: corpus de archivos hostiles (malformados, enormes, encoding raro) → error
  controlado, jamás pánico.

### Migraciones
- Fixture de workspace por cada versión de esquema publicada; migrar N→último y
  verificar invariantes + backup creado.

## 10.3 E2E (Playwright sobre Wails)

Flujos mínimos: crear desde plantilla → editar bloques → validar → exportar MD;
buscar entre 1 000 prompts; drag&drop de bloques; restaurar versión; componer 3 capas;
cambiar tema; atajos principales; simular contra provider mock; consentimiento de plugin.
Matriz: Linux (CI headless) en cada PR; Windows/macOS en nightly y release.

## 10.4 Rendimiento (presupuestos en CI)

Workspace sintético generado (`scripts/genws`): 10 000 prompts, 500 plantillas, 50 tags.

| Métrica | Presupuesto |
|---|---|
| Arranque en frío (core listo) | < 2 s |
| Búsqueda FTS p95 | < 50 ms |
| Indexación completa 10k | < 30 s |
| Reindexado incremental (1 archivo) | < 100 ms |
| Render de prompt 50 KB con 30 variables | < 10 ms |
| Memoria con 10k prompts abiertos en biblioteca | < 300 MB |

Regresión sobre presupuesto = build roja.

## 10.5 CI

- Cada PR: lint (golangci-lint, ruff, mypy), unit + integración (Linux), E2E Linux,
  benchmarks con presupuesto, verificación de reglas de imports entre capas.
- Nightly: matriz completa 3 SOs, E2E completos, tests de migración.
- Release: todo lo anterior + build firmado + smoke manual guiado (checklist).

## 10.6 Calidad más allá de tests

- Fuzzing continuo (go fuzz) sobre: parser de front-matter, parser de bloques,
  template engine, importadores.
- `go vet`, `-race` en toda la suite de integración.
- Revisión obligatoria de PR con checklist de arquitectura (dependencias entre anillos).
