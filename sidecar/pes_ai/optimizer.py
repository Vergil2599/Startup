"""Optimizer determinista de pes-ai.

Genera sugerencias de mejora sobre el IR de un prompt sin necesidad de red ni
de un LLM. Cada sugerencia es una propuesta que el usuario aplica o descarta:
el optimizer JAMÁS modifica el prompt por su cuenta.
"""
from __future__ import annotations

import re

VAGUE_VERBS = (
    "mejorar", "optimizar", "ayudar", "apoyar", "potenciar", "enriquecer",
    "improve", "optimize", "help", "enhance", "assist", "support",
)
FILLER_PATTERNS = (
    (re.compile(r"(?i)\bpor favor,?\s*"), ""),
    (re.compile(r"(?i)\bme gustaría que\b"), "debes"),
    (re.compile(r"(?i)\bsi es posible,?\s*"), ""),
    (re.compile(r"(?i)\bplease,?\s*"), ""),
    (re.compile(r"(?i)\bI would like you to\b"), "you must"),
)
MAX_SENTENCE_WORDS = 45


def _sentences(text: str):
    return [s.strip() for s in re.split(r"[.!?\n]", text) if s.strip()]


def _norm(sentence: str) -> str:
    return re.sub(r"\s+", " ", sentence.strip().lower().strip("-*•· \t"))


def suggest(prompt: dict) -> list[dict]:
    """Analiza el IR del prompt y devuelve sugerencias ordenadas por bloque."""
    out: list[dict] = []
    enabled = [b for b in prompt.get("blocks", []) if b.get("enabled")]

    # 1) Redundancia entre bloques: la misma frase repetida en varios sitios.
    seen: dict[str, str] = {}
    for block in enabled:
        for sentence in _sentences(block.get("content", "")):
            key = _norm(sentence)
            if len(key.split()) < 4:
                continue
            if key in seen and seen[key] != block["type"]:
                out.append({
                    "block_type": block["type"],
                    "kind": "redundancy",
                    "message": (
                        f"la frase «{sentence[:60]}…» ya aparece en el bloque "
                        f"{seen[key]}; repetirla no la refuerza"
                    ),
                })
            else:
                seen.setdefault(key, block["type"])

    # 2) Frases demasiado largas: proponer división.
    for block in enabled:
        for sentence in _sentences(block.get("content", "")):
            if len(sentence.split()) > MAX_SENTENCE_WORDS:
                out.append({
                    "block_type": block["type"],
                    "kind": "clarity",
                    "message": f"frase de {len(sentence.split())} palabras; divídela o usa viñetas",
                })
                break  # una por bloque

    # 3) Objetivo vago sin criterio medible.
    for block in enabled:
        if block["type"] != "objective":
            continue
        content = block.get("content", "")
        lower = content.lower()
        has_metric = bool(re.search(r"\d|%|criteri|métrica|metric", lower))
        for verb in VAGUE_VERBS:
            if verb in lower and not has_metric:
                out.append({
                    "block_type": "objective",
                    "kind": "precision",
                    "message": f"el verbo «{verb}» es vago: define qué resultado concreto esperas",
                    "replacement_hint": "añade una métrica o un bloque success_criteria",
                })
                break

    # 4) Relleno cortés: instrucciones directas funcionan mejor.
    for block in enabled:
        content = block.get("content", "")
        rewritten = content
        hit = False
        for pattern, repl in FILLER_PATTERNS:
            if pattern.search(rewritten):
                rewritten = pattern.sub(repl, rewritten)
                hit = True
        if hit:
            out.append({
                "block_type": block["type"],
                "kind": "directness",
                "message": "elimina fórmulas de cortesía: las instrucciones directas son más fiables",
                "replacement": re.sub(r"  +", " ", rewritten).strip(),
            })

    # 5) Sin ejemplos cuando se exige formato de salida.
    types = {b["type"] for b in enabled}
    if "output" in types and "examples" not in types:
        output_block = next(b for b in enabled if b["type"] == "output")
        if re.search(r"(?i)json|yaml|tabla|table|formato|format|lista|list", output_block.get("content", "")):
            out.append({
                "block_type": "examples",
                "kind": "structure",
                "message": "el Output exige un formato: un bloque Examples con un caso concreto reduce errores",
            })

    return out
