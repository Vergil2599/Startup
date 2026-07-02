# ADR-004: Sidecar Python (`pes-ai`) por gRPC sobre socket local

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
La IA es estrictamente opcional, pero el Optimizer, la evaluación de calidad y ciertos
adaptadores LLM se benefician del ecosistema Python.

## Decisión
Proceso Python separado y opcional que sirve gRPC sobre Unix socket / named pipe;
el core Go define los puertos (`Optimizer`, `LLMProvider`) y el sidecar es una
implementación más. Sin handshake exitoso, las features de IA se marcan no disponibles.

## Consecuencias
+ El binario Go sigue siendo autosuficiente; "offline puro" es un estado de primera clase.
+ Contrato único protobuf en `proto/` generado para ambos lados (anti-deriva).
− Distribución del sidecar añade trabajo (PyInstaller por SO, instalación guiada).

## Alternativas descartadas
- **Embeber CPython en Go**: frágil, CGO, pesadilla de distribución.
- **Todo en Go**: pierde el ecosistema IA de Python para optimización/evaluación.
- **REST local**: sin streaming tipado ni contrato generado.
- **Puerto TCP**: superficie de ataque innecesaria; el socket local no la tiene.
