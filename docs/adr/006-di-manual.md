# ADR-006: Inyección de dependencias manual por constructores

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
Los requisitos piden SOLID, Clean Architecture y DI, con bajo acoplamiento y testabilidad.

## Decisión
DI manual: todas las dependencias entran por constructor como interfaces definidas en
`internal/core/ports.go`; el cableado completo vive en un único paquete `internal/app`.

## Consecuencias
+ Grafo de dependencias explícito y navegable; cero magia ni reflexión.
+ Tests sustituyen puertos por fakes sin framework.
− `internal/app` crece con el proyecto (aceptable: es el único lugar de wiring).

## Alternativas descartadas
- **google/wire**: codegen que aporta poco a esta escala.
- **uber/fx**: contenedor por reflexión, errores en runtime en vez de compilación.
