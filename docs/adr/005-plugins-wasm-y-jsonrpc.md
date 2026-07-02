# ADR-005: Plugins en dos runtimes — WASM (wazero) y procesos JSON-RPC/stdio

- **Estado**: Propuesto (pendiente de aprobación de la arquitectura)
- **Fecha**: 2026-07-02

## Contexto
La extensibilidad por plugins es un principio del producto, con seguridad por defecto y
soporte de múltiples lenguajes (al menos Go y Python).

## Decisión
Dos runtimes con el mismo catálogo de extension points: WASM in-process (wazero, sandbox
por capacidades) para extensiones de cómputo, y procesos externos JSON-RPC 2.0 por stdio
para integraciones e I/O. Capacidades denegadas por defecto y consentimiento explícito.

## Consecuencias
+ Sandbox real para código de terceros; Python de primera clase vía proceso.
+ wazero es Go puro (sin CGO); pánico del plugin no tumba el host.
− Mantener dos runtimes y una normalización común (`promptIR`) tiene costo.

## Alternativas descartadas
- **`plugin` de la stdlib de Go**: solo Linux/macOS, sin sandbox, versiones frágiles.
- **Solo procesos**: sin sandbox fino para plugins de cómputo puro y mayor latencia.
- **Scripting embebido (Lua/JS)**: tercer lenguaje fuera de la restricción Go+Python.
