# 14. Estrategia de sincronización futura

## 14.1 Postura

La sincronización **no es requisito de ninguna versión ≤ v1** y jamás será obligatoria.
El diseño actual (archivos como fuente de verdad, IDs estables ULID, snapshots
content-addressed, detección de conflictos por hash) se eligió deliberadamente para que
la sincronización futura sea una **capa aditiva**, sin re-arquitectura.

## 14.2 Fases

### Fase 0 (ya cubierta por el diseño actual): "sincronización manual"
El workspace es una carpeta normal → el usuario puede usar **git, Syncthing, Dropbox o
iCloud hoy mismo**, sin soporte especial. El watcher + diálogo de conflictos de 3 vías
([13-persistencia.md §13.6](13-persistencia.md)) ya absorbe los cambios externos.
Documentaremos estos flujos como recetas oficiales.

### Fase 1 (v2): sincronización git-backed integrada (plugin first-party)
- El plugin Git/GitHub convierte el workspace en repo: commit automático por snapshot,
  push/pull desde la UI, historial de PES ↔ historial git correlacionados.
- Conflictos: se reutiliza el merge por bloques del Composer como driver de merge
  (`.gitattributes` + `pes merge-driver`), con fallback al diálogo de 3 vías.
- Sirve equipos completos (revisión de prompts por PR) sin servidor propio.
- Riesgo bajo: no inventamos protocolo; git es la capa de transporte y auth.

### Fase 2 (v2.x, investigación): sync E2E-encrypted propio
Solo si la demanda lo justifica. Requisitos de diseño ya fijados:

- **E2E por defecto**: el servidor (autohospedable) solo ve blobs cifrados (age/XChaCha20);
  claves solo en los dispositivos.
- **Modelo de datos**: log de operaciones por prompt (crear/actualizar bloque, mover,
  taggear…) sobre los snapshots content-addressed existentes; los IDs ULID + hashes
  actuales son suficientes como base de un CRDT simple por archivo
  (LWW por bloque + merge estructural; el contenido de un bloque en conflicto real
  se resuelve con el diálogo de 3 vías, no automáticamente).
- **Transporte**: HTTPS simple, pull/push de blobs + log; sin conexión persistente
  requerida (sigue siendo offline-first: la cola de salida espera).
- **Servidor de referencia**: binario Go autohospedable, almacenamiento en disco/S3-compat.

## 14.3 Decisiones de hoy que protegen el mañana

| Decisión ya tomada | Por qué habilita sync |
|---|---|
| IDs ULID estables e independientes de la ruta | renombrar/mover no rompe identidad entre réplicas |
| Snapshots content-addressed (SHA-256) | deduplicación y transferencia incremental triviales |
| Conflictos detectados por `content_hash`, nunca autoresueltos silenciosamente | la UX de conflicto ya existe; sync solo añade un origen más de cambios |
| Bloques tipados y ordenados como unidad de merge | merge estructural con menos falsos conflictos que texto plano |
| Event bus interno | un motor de sync se suscribe a `PromptSaved` sin tocar servicios |
| `.pes/` separado del contenido | lo derivado (índice, cachés) nunca se sincroniza |

## 14.4 Qué NO haremos

- Sync con resolución automática de conflictos de contenido (silenciosa): prohibido por
  principio de "nunca perder trabajo".
- Cuentas obligatorias, telemetría acoplada al sync, o sync como default activado.
- Formato de red propietario antes de agotar la vía git.
