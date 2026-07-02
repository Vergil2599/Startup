# ADR-001: Wails v2 como shell de escritorio

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
PES es un IDE de escritorio multiplataforma. La restricción de lenguajes es Go+Python,
pero un IDE necesita UI rica (paneles acoplables, drag&drop, editor con resaltado).

## Decisión
Usar Wails v2: ventana nativa con el WebView del SO, toda la lógica en Go expuesta por
bindings, capa de vista web deliberadamente fina.

## Consecuencias
+ Binario ligero (~15 MB) frente a Electron; sin runtime Node; lógica 100 % en Go.
+ La CLI comparte exactamente los mismos casos de uso: garantía anti-filtración de lógica.
− WebViews difieren por SO → matriz E2E por plataforma obligatoria (riesgo R1).

## Alternativas descartadas
- **Electron**: peso, RAM, y añadiría Node al stack.
- **Fyne/Gio (Go puro)**: widgets insuficientes para un IDE (dock, editor rico, DnD complejo).
- **templ+HTMX 100 % Go**: se mantiene como plan B documentado; el round-trip por binding
  en cada interacción degrada un editor rico.
