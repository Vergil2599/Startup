# 13. Estrategia de persistencia

## 13.1 Principios

1. **Los datos del usuario son archivos del usuario**: legibles, editables fuera de la
   app, versionables con git, compatibles con Obsidian. Sin formatos opacos.
2. **Fuente de verdad única**: el workspace en disco. Todo lo demás (índice, cachés)
   es derivado y desechable.
3. **Durabilidad ante crash**: ninguna operación deja el workspace en estado corrupto.
4. **Offline total**: cero dependencia de servicios; la sincronización (futura) es una
   capa encima, nunca un requisito ([14-sincronizacion.md](14-sincronizacion.md)).

## 13.2 Capas de persistencia

| Capa | Qué guarda | Formato | ¿Desechable? |
|---|---|---|---|
| **Workspace** | prompts, plantillas, variables, composiciones | MD+YAML / YAML | **No** (fuente de verdad) |
| `.pes/history/` | snapshots de versiones | archivos content-addressed (SHA-256) + índice | No (es el historial) |
| `.pes/runs/` | respuestas de simulador/benchmark | JSON por ejecución | Purgable por retención |
| `.pes/index.db` | índice de búsqueda y metadatos | SQLite FTS5 | **Sí** (`pes reindex`) |
| `.pes/backups/` | backups rotados del workspace | tar.zst (+age opcional) | Rotación automática |
| Config global | settings de la app, plugins | TOML en dir. de config del SO | Parcial |
| Credenciales | API keys de providers | keyring del SO; fallback archivo age | No |

## 13.3 Escrituras seguras

- **Atómicas siempre**: escribir a `archivo.md.tmp` → `fsync` → `rename` → `fsync` del
  directorio. En Windows, `ReplaceFile` para preservar atributos.
- Una operación de usuario = una escritura de archivo como máximo por prompt
  (los bloques viven dentro del archivo del prompt; no hay estados intermedios multi-archivo).
- Operaciones multi-archivo (migraciones, restore masivo) crean backup previo y journal
  de operación en `.pes/journal` para recuperación al arrancar tras un crash.

## 13.4 Autoguardado, undo e historial — tres niveles distintos

| Nivel | Ámbito | Persistencia |
|---|---|---|
| **Undo/redo** ilimitado | edición en memoria por prompt (sesión) | pila serializada en `.pes/session/` para sobrevivir reinicios |
| **Autoguardado** | disco, debounce 500 ms tras pausa de tipeo | el archivo .md siempre refleja el estado casi actual |
| **Versiones (snapshots)** | hitos con autor+comentario | manuales (`Ctrl+S`) + automáticos por política (cada N minutos de edición efectiva / antes de restore, merge, migración, optimización IA) |

Snapshots deduplicados por hash de contenido: versionar sin cambios no ocupa espacio.

## 13.5 Índice y caché

- Reindexado **incremental**: comparación `mtime` + `content_hash` al arrancar y ante
  eventos del watcher; reconstrucción total disponible siempre.
- Caché de validación por `content_hash` (tabla `validation_cache`).
- Caché LRU en memoria de prompts parseados (los cuerpos se cargan de forma diferida;
  las listas usan solo el índice).

## 13.6 Ediciones externas y conflictos

- Watcher (fsnotify) sobre el workspace. Cambio externo en un prompt **no abierto** →
  reindexar y listo. En un prompt **abierto con cambios locales** → conflicto:
  diálogo de 3 vías (mío / externo / merge por bloques). Nunca se resuelve en silencio.
- El lock del workspace es advisory (`.pes/lock` con PID): dos instancias de PES sobre el
  mismo workspace → la segunda abre en solo-lectura con aviso.

## 13.7 Backups

- Automáticos: al abrir el workspace (máx. 1/día) y antes de operaciones peligrosas
  (migración, restore masivo, aplicar optimización a muchos prompts).
- Rotación GFS simplificada: 7 diarios, 4 semanales, 6 mensuales (configurable).
- Formato `tar.zst`; con cifrado age si el usuario activó cifrado.
- Restauración guiada desde Ajustes → "Copias de seguridad" (previsualiza contenido antes).

## 13.8 Cifrado opcional

- **Workspace cifrado**: modo opt-in en el que los archivos se almacenan cifrados con age
  (passphrase o identity file); la app descifra en memoria. Trade-off explícito al usuario:
  pierde interoperabilidad directa con git/Obsidian sobre esos archivos.
- **Credenciales**: siempre cifradas (keyring del SO; fallback archivo age), nunca en TOML/YAML.
- Los backups heredan el modo de cifrado del workspace.

## 13.9 Retención y límites

- `runs/`: retención configurable (por defecto 90 días o 500 MB, lo que llegue antes).
- `history/`: sin límite por defecto (es pequeño gracias a deduplicación); compactación
  manual disponible.
- Purga siempre con confirmación y nunca incluye la fuente de verdad.
