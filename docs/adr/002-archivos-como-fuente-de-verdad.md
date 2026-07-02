# ADR-002: Archivos Markdown+YAML como fuente de verdad

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
Offline-first, sin telemetría, interoperabilidad con git/Obsidian y durabilidad a años
son principios del producto.

## Decisión
Cada prompt/plantilla es un archivo Markdown con front-matter YAML en una carpeta del
usuario; todo almacenamiento adicional (índice SQLite, cachés) es derivado y desechable.

## Consecuencias
+ Git-friendly, editable externamente, transparente, imposible de "secuestrar" en un formato opaco.
+ La corrupción del índice nunca pierde datos (`pes reindex`).
− Hay que gestionar ediciones externas y conflictos (watcher + diálogo de 3 vías).
− Consultas complejas requieren mantener el índice sincronizado (mtime+hash).

## Alternativas descartadas
- **Solo SQLite**: rápido pero opaco, hostil a git/Obsidian, riesgo de lock-in propio.
- **Un archivo por bloque**: escrituras multi-archivo no atómicas y ruido en git.
