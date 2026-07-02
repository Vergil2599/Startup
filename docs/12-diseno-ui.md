# 12. Diseño de UI

## 12.1 Layout maestro

Estilo IDE, familiar para el público objetivo (usuarios de VS Code/Cursor/Obsidian):

```
┌──┬──────────────┬──────────────────────────────┬─────────────┐
│A │  Sidebar     │   Zona de editores           │ Panel dcho  │
│c │  (Biblioteca │   (tabs, editor dividido)    │ (Preview /  │
│t │   árbol+tags │                              │  Validación │
│i │   +búsqueda) │  ┌─ Bloques del prompt ────┐ │  / Runner / │
│v │              │  │ [Role      ] [on] ⋮≡    │ │  Historial) │
│i │              │  │ [Objective ] [on] ⋮≡    │ │             │
│d │              │  │ [Rules     ] [off] ⋮≡   │ │             │
│a │              │  └─────────────────────────┘ │             │
│d ├──────────────┴──────────────────────────────┴─────────────┤
│  │  Panel inferior: Problemas · Ejecuciones · Log de plugins │
├──┴────────────────────────────────────────────────────────────┤
│ Status bar: workspace · score del prompt · provider · sync    │
└────────────────────────────────────────────────────────────────┘
```

Todos los paneles son acoplables/colapsables; el estado del layout se persiste.

## 12.2 Sistema de diseño

- **Design tokens** (CSS custom properties) como única fuente de estilo: color,
  espaciado (escala de 4 px), radios, tipografía, sombras, duración de animaciones.
  Los temas solo redefinen tokens.
- **Tipografía**: Inter (UI) + JetBrains Mono (contenido de bloques, diffs, respuestas).
  Escala: 12/13/14/16/20/24.
- **Densidad**: modo cómodo (por defecto) y compacto (ajuste global).
- **Iconos**: Lucide (outline, consistente, licencia permisiva).

## 12.3 Temas

| Token (muestra) | Claro | Oscuro |
|---|---|---|
| `--bg-base` | `#FAFAF8` | `#1B1D23` |
| `--bg-panel` | `#FFFFFF` | `#22252D` |
| `--fg-primary` | `#1A1C22` | `#E8E9EC` |
| `--accent` | `#4F46E5` | `#818CF8` |
| `--ok / --warn / --error` | `#16A34A / #D97706 / #DC2626` | `#4ADE80 / #FBBF24 / #F87171` |

Contraste AA verificado en CI (test de tokens). El tema sigue al SO por defecto,
con override manual. Los temas de terceros llegan como plugins (solo tokens, sin CSS arbitrario).

## 12.4 Componentes clave

### BlockCard (corazón del builder)
- Cabecera: icono + nombre del tipo, toggle on/off, asa de arrastre (`≡`), menú (`⋮`),
  chevron de colapso. Bloques desactivados: opacidad reducida + patrón rayado sutil.
- Cuerpo: editor CodeMirror con resaltado de `{{variables}}` (chip coloreado por ámbito:
  global=azul, proyecto=verde, prompt=morado, sin definir=rojo subrayado).
- Pie contextual: contador de tokens estimado del bloque + findings de validación propios.

### ScoreBadge
Anillo de progreso 0–100 con color semántico (rojo <50, ámbar 50–79, verde ≥80).
Clic abre el desglose. Animación de transición suave (respeta reduced-motion).

### VariableChip / VariablePanel
Chips inline en el editor; panel lateral con tabla nombre/valor/ámbito editable,
indicador de "usada en N prompts" y renombrado seguro (F2).

### DiffView (comparador e historial)
Dos modos: **texto** (líneas, estilo git) y **estructural** (tarjetas de bloque:
verde=añadido, rojo=eliminado, ámbar=modificado, gris=movido) + barra de estadísticas
(bloques ±, palabras ±, variables ±, Δ score).

### RunnerPanel
Cabecera con selector provider/modelo/parámetros (presets guardables). Respuesta en
streaming con métricas en vivo (tokens/s, tiempo). Modo comparación: N columnas
sincronizadas en scroll. Benchmark: tabla ordenable + mini-gráficas de barras por métrica.

### CommandPalette
Overlay central (Ctrl+Shift+P), búsqueda difusa, agrupada por módulo, muestra atajos;
los plugins aportan comandos con su icono y prefijo del plugin.

### Onboarding/EmptyStates
Ilustraciones ligeras (SVG monocromo con acento), un CTA primario por estado, tono
directo ("Crea tu primer prompt desde una plantilla").

## 12.5 Interacción y movimiento

- Drag & drop con **preview fantasma** y línea de inserción de 2 px en acento; auto-scroll
  en bordes; siempre existe equivalente de teclado.
- Animaciones 120–180 ms, solo transform/opacity (sin layout thrash); desactivables.
- Toasts no bloqueantes abajo-derecha (guardado, export, errores de provider) con acción
  de deshacer cuando aplique.

## 12.6 Responsive dentro del escritorio

Ancho mínimo soportado 1024 px: por debajo de 1280 px el panel derecho se convierte en
tabs del panel inferior; la sidebar es colapsable a iconos (rail). Sin versión móvil en alcance.
