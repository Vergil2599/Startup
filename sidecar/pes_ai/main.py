#!/usr/bin/env python3
"""pes-ai: sidecar de IA OPCIONAL de Prompt Engineering Studio.

Habla el mismo protocolo que cualquier plugin de PES (JSON-RPC 2.0 por stdio,
un mensaje por línea) y contribuye el extension point "optimizer". PES funciona
al 100 % sin este proceso: si no está, la app marca la optimización como
"no disponible" y nada más cambia.

Solo usa la biblioteca estándar: no requiere pip install.
"""
import json
import sys

# Permitir ejecución tanto como módulo (python -m pes_ai.main) como script.
try:
    from .optimizer import suggest  # type: ignore
except ImportError:  # pragma: no cover
    sys.path.insert(0, __file__.rsplit("/", 2)[0])
    from pes_ai.optimizer import suggest

VERSION = "1.0.0"


def describe(_params):
    return {
        "id": "dev.pes.ai",
        "name": "pes-ai (Optimizer)",
        "version": VERSION,
        "extension_points": ["optimizer"],
    }


def optimizer_suggest(params):
    prompt = params.get("prompt")
    if not isinstance(prompt, dict):
        raise ValueError("params.prompt (IR) es obligatorio")
    return {"suggestions": suggest(prompt)}


METHODS = {
    "plugin.describe": describe,
    "optimizer.suggest": optimizer_suggest,
}


def handle(req: dict) -> dict:
    req_id = req.get("id")
    handler = METHODS.get(req.get("method", ""))
    if handler is None:
        return {"jsonrpc": "2.0", "id": req_id,
                "error": {"code": -32601, "message": "método desconocido"}}
    try:
        return {"jsonrpc": "2.0", "id": req_id, "result": handler(req.get("params") or {})}
    except Exception as exc:  # noqa: BLE001 - el sidecar nunca debe morir
        return {"jsonrpc": "2.0", "id": req_id,
                "error": {"code": -32000, "message": str(exc)}}


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError:
            continue
        sys.stdout.write(json.dumps(handle(req)) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
