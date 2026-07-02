# 9. Riesgos técnicos y mitigaciones

Escala: Probabilidad (P) y Impacto (I) en Bajo/Medio/Alto.

## 9.1 Riesgos de plataforma

| # | Riesgo | P | I | Mitigación |
|---|---|---|---|---|
| R1 | **WebView inconsistente** entre Windows (WebView2), macOS (WKWebView) y Linux (WebKitGTK): bugs de render, APIs faltantes | M | A | CSS/JS conservador; matriz E2E por SO en CI; smoke test manual por release; plan B documentado (templ+HTMX server-side dentro del shell) |
| R2 | **Wails v2** cambia de rumbo (v3) o pierde mantenimiento | M | M | La vista solo consume bindings → migrar de shell no toca core; CLI ya cubre el 100 % de casos de uso |
| R3 | Distribución multiplataforma (firma macOS, SmartScreen Windows) | A | M | Pipeline de firma/notarización desde v0.1; instrucciones claras para builds sin firmar |

## 9.2 Riesgos de datos

| # | Riesgo | P | I | Mitigación |
|---|---|---|---|---|
| R4 | **Corrupción/pérdida de archivos** del workspace (crash a mitad de escritura) | B | A | Escritura atómica (tmp+rename+fsync); snapshots content-addressed; backups rotados automáticos; el índice nunca es fuente de verdad |
| R5 | **Ediciones externas concurrentes** (git pull, Obsidian abierto a la vez) | A | M | Watcher fsnotify + detección de conflicto por `content_hash`; diálogo de 3 vías (mío/externo/merge por bloques); nunca sobrescribir en silencio |
| R6 | Migraciones de esquema de archivo rompen workspaces antiguos | M | A | Campo `pes: N`, migradores idempotentes con backup previo obligatorio, tests de migración con fixtures de cada versión publicada |
| R7 | FTS5 degrada con workspaces enormes (50k+) | B | M | Índice incremental por mtime/hash; presupuestos de rendimiento en CI; paginación y carga diferida de cuerpos |

## 9.3 Riesgos del sidecar Python / IA

| # | Riesgo | P | I | Mitigación |
|---|---|---|---|---|
| R8 | **Fricción de instalación de Python** en usuarios no técnicos | A | M | Binarios PyInstaller por SO; detección + instalación guiada; y sobre todo: **todo funciona sin el sidecar** |
| R9 | Deriva de contrato Go↔Python | M | M | Protobuf único en `proto/`, generación en CI, tests de contrato que arrancan el sidecar real |
| R10 | Optimizer da sugerencias malas y erosiona confianza | M | M | Sugerencias siempre como **diff aplicable por bloque**, nunca autoaplicadas; etiquetadas como IA; feedback de descarte |
| R11 | Cambios de APIs de proveedores LLM | A | B | Providers como plugins actualizables fuera del ciclo de la app; tests contra mocks + smoke opcionales contra servicios reales |

## 9.4 Riesgos del sistema de plugins

| # | Riesgo | P | I | Mitigación |
|---|---|---|---|---|
| R12 | **Plugin malicioso o defectuoso** accede a datos del usuario | M | A | Capacidades denegadas por defecto + consentimiento explícito; WASM sandbox; procesos con circuit breaker; firmas en marketplace (v2) |
| R13 | API de plugins mal diseñada obliga a breaking changes tempranos | M | A | La API se "come su propia comida": exportadores y providers builtin usan los mismos extension points desde v1; beta pública de la API antes de declararla estable |
| R14 | Plugin lento bloquea la UX | M | M | Timeouts por invocación, ejecución fuera del hilo de UI, atribución de latencia visible en el gestor de plugins |

## 9.5 Riesgos de producto/alcance

| # | Riesgo | P | I | Mitigación |
|---|---|---|---|---|
| R15 | **Scope creep** (es "un VS Code para prompts": tentación infinita) | A | A | Roadmap con criterios de salida medibles; regla "plugins, no core" para toda feature nueva; MVP sin IA |
| R16 | El diff/merge por bloques (Composer, conflictos) es más difícil de lo previsto | M | M | Spike técnico en M0 con property-based testing (rapid); estrategia `concat` simple como fallback siempre disponible |
| R17 | Puntuación del validador percibida como arbitraria | M | M | Desglose explicable por categoría, severidades configurables, umbral por proyecto; documentar cada regla |
| R18 | Un solo desarrollador / bus factor | A | M | Esta documentación + ADRs + CI estricta; módulos pequeños con contratos claros |

## 9.6 Señales de alarma tempranas (a vigilar en cada release)

- Arranque > 2 s o búsqueda > 50 ms p95 en el workspace de referencia (10k prompts).
- Cualquier feature nueva que requiera red para funcionar.
- Lógica de negocio apareciendo en `ui/` (detectable por revisión + lint de imports).
- Issues de pérdida de datos, aunque sean únicos: prioridad máxima automática.
