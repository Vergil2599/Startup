# ADR-003: SQLite FTS5 (driver CGO-free) como índice de búsqueda

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
Requisito: búsqueda instantánea (<50 ms) sobre miles de prompts, offline, sin servicios.

## Decisión
Índice SQLite con FTS5 vía `modernc.org/sqlite` (Go puro, sin CGO), reconstruible
íntegramente desde el workspace.

## Consecuencias
+ Full-text probado, un solo archivo, cross-compilación trivial sin CGO.
+ Sirve también para metadatos consultables (tags, versiones, runs).
− Rendimiento algo menor que el driver CGO (aceptable: es un índice local).

## Alternativas descartadas
- **Bleve**: más memoria, menos maduro, segundo motor que aprender.
- **mattn/go-sqlite3**: CGO complica la matriz de builds.
- **Índice en memoria propio**: reinventar FTS (stemming, unicode, ranking).
