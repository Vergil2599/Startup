#!/usr/bin/env python3
"""Plugin de ejemplo de PES en Python puro (stdlib, sin dependencias).

Protocolo: JSON-RPC 2.0 por stdio, un mensaje JSON por línea.
Contribuye:
  - validation.rule: detecta bloques activos con más de `max_words` palabras.
  - llm.provider:    proveedor de eco (útil para probar el simulador sin red).
"""
import json
import sys

MAX_WORDS = 300


def describe(_params):
    return {
        "id": "dev.pes.example-python",
        "name": "Ejemplo Python",
        "version": "0.1.0",
        "extension_points": ["validation.rule", "llm.provider"],
    }


def validation_check(params):
    findings = []
    prompt = params.get("prompt", {})
    for block in prompt.get("blocks", []):
        if not block.get("enabled"):
            continue
        words = len(block.get("content", "").split())
        if words > MAX_WORDS:
            findings.append({
                "severity": "warn",
                "block_type": block.get("type", ""),
                "message": f"el bloque tiene {words} palabras (>{MAX_WORDS})",
                "suggestion": "divide el contenido o muévelo a otro bloque",
            })
    return {"findings": findings}


def llm_complete(params):
    prompt = params.get("prompt", "")
    return {
        "text": f"[eco desde plugin] {prompt[:200]}",
        "tokens_in": len(prompt.split()),
        "tokens_out": min(len(prompt.split()), 50),
    }


METHODS = {
    "plugin.describe": describe,
    "validation.check": validation_check,
    "llm.complete": llm_complete,
}


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            req = json.loads(line)
        except json.JSONDecodeError:
            continue
        req_id = req.get("id")
        method = req.get("method", "")
        handler = METHODS.get(method)
        if handler is None:
            resp = {"jsonrpc": "2.0", "id": req_id,
                    "error": {"code": -32601, "message": f"método desconocido: {method}"}}
        else:
            try:
                resp = {"jsonrpc": "2.0", "id": req_id,
                        "result": handler(req.get("params") or {})}
            except Exception as exc:  # noqa: BLE001 - el plugin nunca debe morir
                resp = {"jsonrpc": "2.0", "id": req_id,
                        "error": {"code": -32000, "message": str(exc)}}
        sys.stdout.write(json.dumps(resp) + "\n")
        sys.stdout.flush()


if __name__ == "__main__":
    main()
