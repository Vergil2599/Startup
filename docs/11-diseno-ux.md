# 11. Diseño de UX

## 11.1 Principios de experiencia

1. **Progresión sin muros**: el principiante ve plantillas y un builder guiado; el experto
   vive en la paleta de comandos y los atajos. Ninguna función avanzada estorba al novato.
2. **Nunca perder trabajo**: autoguardado continuo, undo/redo ilimitado, historial visible.
3. **Feedback inmediato**: puntuación de validación y vista previa se actualizan mientras
   se escribe (debounce 300 ms), sin botón "validar".
4. **Todo alcanzable por teclado**; el ratón (drag&drop) es un acelerador, no un requisito.
5. **La IA se ofrece, no se impone**: las sugerencias del Optimizer aparecen como
   propuestas diferenciadas visualmente, aplicables por bloque, nunca automáticas.

## 11.2 Mapa de navegación

```
Barra de actividad (izquierda, estilo VS Code)
├── 📚 Biblioteca        (árbol carpetas + tags + favoritos + búsqueda)
├── ✏️  Builder           (editor por bloques del prompt activo)
├── 🧩 Composer          (capas → prompt final)
├── 🔀 Comparador        (diff entre prompts o versiones)
├── ▶️  Runner            (simulador + benchmark)
├── 🧩 Plugins           (gestor, capacidades, estado)
└── ⚙️  Ajustes
```

Paneles acoplables (dock): cualquier vista puede anclarse a izquierda/derecha/abajo o
flotar; layouts guardados por el usuario. **Editor dividido**: dos prompts lado a lado
(o prompt + vista previa, o prompt + resultados del runner).

## 11.3 Flujos clave

### Primer uso (onboarding, < 5 min)
1. Elegir/crear carpeta de workspace (explicación: "tus prompts son archivos tuyos").
2. Tour de 4 pasos sobre la plantilla "Mi primer prompt" precargada.
3. Editar bloque Objective → ver la puntuación subir → exportar a clipboard. Fin.
Sin registro, sin red, sin configurar ningún modelo.

### Crear un prompt
`Ctrl+N` → selector: en blanco | desde plantilla (por categoría, con preview) | importar.
El builder muestra los bloques de la plantilla; los no incluidos se añaden desde un
catálogo lateral (drag o `Ctrl+Shift+B`). Cada bloque: toggle on/off, colapsar,
reordenar (drag o `Alt+↑/↓`), menú contextual (duplicar, convertir tipo, extraer a plantilla).

### Variables
Al escribir `{{` se abre autocompletado con variables existentes (por ámbito, con su valor
actual). Variables sin definir se subrayan y aparecen en el panel de problemas; un clic
las crea en el ámbito elegido. La vista previa muestra el prompt renderizado en tiempo real.

### Validación
Badge de puntuación (0–100) siempre visible en la cabecera del builder. Clic → panel de
findings agrupado por severidad; cada finding enlaza al bloque y ofrece sugerencia
("Objective usa el verbo vago 'mejorar' — define una métrica"). Umbral configurable por
proyecto para marcar un prompt como "listo".

### Componer
Vista Composer: columna de capas ordenables (base + overlays), toggle por capa,
estrategia de merge visible, y a la derecha el resultado en vivo con origen de cada
bloque coloreado por capa. "Materializar" crea un prompt normal vinculado a su composición.

### Probar (simulador)
Desde el builder: `Ctrl+Enter` → panel Runner con el prompt renderizado, selector de
provider/modelo/parámetros, respuesta en streaming. Historial de ejecuciones por prompt.
Benchmark: seleccionar N modelos + repeticiones → tabla comparativa (tiempo, tokens,
costo, consistencia) con export a MD/HTML.

### Historial y comparación
Timeline lateral por prompt (versión, autor, comentario). Seleccionar dos →
comparador con diff por bloques (añadidos/eliminados/modificados) + estadísticas.
"Restaurar" crea una versión nueva (nunca reescribe historia).

## 11.4 Atajos de teclado (núcleo)

| Atajo | Acción |
|---|---|
| `Ctrl+Shift+P` | Paleta de comandos (toda acción es un comando, también las de plugins) |
| `Ctrl+P` | Ir a prompt (búsqueda difusa) |
| `Ctrl+N` / `Ctrl+S` | Nuevo / guardar (el guardado es automático; S fuerza snapshot con comentario) |
| `Ctrl+Enter` | Ejecutar en simulador |
| `Ctrl+Shift+V` | Alternar vista previa renderizada |
| `Ctrl+Shift+B` | Añadir bloque |
| `Alt+↑/↓` | Mover bloque |
| `Ctrl+E` | Exportar… |
| `Ctrl+Z` / `Ctrl+Shift+Z` | Undo / redo (ilimitado, por prompt) |
| `Ctrl+\` | Dividir editor |
| `F2` | Renombrar (prompt, variable — renombrado de variable propaga a usos) |

Todos los atajos son remapeables en Ajustes.

## 11.5 Estados vacíos y errores

- Biblioteca vacía → CTA "crear desde plantilla" con las 3 categorías más comunes.
- Provider no configurado al abrir Runner → tarjeta explicando opciones locales
  (Ollama/LM Studio) con guía de 2 pasos; nunca un formulario de API key como primera opción.
- Sidecar IA ausente → las entradas de Optimizer muestran "IA no instalada (opcional)" con
  enlace a instalación; jamás un error.
- Conflicto de edición externa → diálogo de 3 vías con diff, opciones "mantener el mío /
  tomar el externo / fusionar por bloques"; elección recordable por sesión.

## 11.6 Accesibilidad

- Navegación completa por teclado con orden de foco lógico; roles ARIA en la vista.
- Contraste AA en ambos temas; tamaños de fuente ajustables; respetar
  `prefers-reduced-motion`.
- Textos de la UI externalizados (i18n preparado; ES/EN de partida).
