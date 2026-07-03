"""Tests del optimizer de pes-ai (python3 -m unittest discover sidecar/tests)."""
import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from pes_ai.optimizer import suggest  # noqa: E402
from pes_ai.main import handle  # noqa: E402


def ir(blocks):
    return {"id": "x", "title": "T", "blocks": blocks}


def block(btype, content, enabled=True):
    return {"type": btype, "enabled": enabled, "content": content}


class OptimizerTests(unittest.TestCase):
    def kinds(self, suggestions):
        return [s["kind"] for s in suggestions]

    def test_clean_prompt_has_no_suggestions(self):
        s = suggest(ir([
            block("objective", "Detecta 3 defectos por revisión."),
            block("output", "Texto plano."),
        ]))
        self.assertEqual(s, [])

    def test_redundancy_across_blocks(self):
        phrase = "nunca inventes APIs que no existen"
        s = suggest(ir([
            block("rules", phrase),
            block("constraints", phrase),
        ]))
        self.assertIn("redundancy", self.kinds(s))

    def test_long_sentence_flagged_once_per_block(self):
        long_sentence = " ".join(["palabra"] * 60) + "."
        s = suggest(ir([block("context", long_sentence + " " + long_sentence)]))
        self.assertEqual(self.kinds(s).count("clarity"), 1)

    def test_vague_objective(self):
        s = suggest(ir([block("objective", "Mejorar el código del proyecto.")]))
        self.assertIn("precision", self.kinds(s))
        # Con métrica deja de ser vago.
        s = suggest(ir([block("objective", "Mejorar el código: reducir un 20% los defectos.")]))
        self.assertNotIn("precision", self.kinds(s))

    def test_filler_removed_with_replacement(self):
        s = suggest(ir([block("rules", "Por favor, sigue la guía. Me gustaría que uses tests.")]))
        matches = [x for x in s if x["kind"] == "directness"]
        self.assertEqual(len(matches), 1)
        self.assertNotIn("Por favor", matches[0]["replacement"])
        self.assertIn("debes", matches[0]["replacement"])

    def test_examples_suggested_for_formatted_output(self):
        s = suggest(ir([
            block("objective", "Genera el informe con 3 secciones."),
            block("output", "Devuelve un JSON con claves fijas."),
        ]))
        self.assertIn("structure", self.kinds(s))

    def test_disabled_blocks_ignored(self):
        s = suggest(ir([block("objective", "Mejorar todo.", enabled=False)]))
        self.assertEqual(s, [])


class RPCTests(unittest.TestCase):
    def test_describe(self):
        resp = handle({"jsonrpc": "2.0", "id": 1, "method": "plugin.describe"})
        self.assertEqual(resp["result"]["id"], "dev.pes.ai")
        self.assertIn("optimizer", resp["result"]["extension_points"])

    def test_suggest_roundtrip(self):
        resp = handle({"jsonrpc": "2.0", "id": 2, "method": "optimizer.suggest",
                       "params": {"prompt": ir([block("objective", "Ayudar al usuario.")])}})
        self.assertNotIn("error", resp)
        self.assertTrue(resp["result"]["suggestions"])

    def test_bad_params_is_error_not_crash(self):
        resp = handle({"jsonrpc": "2.0", "id": 3, "method": "optimizer.suggest",
                       "params": {"prompt": "no soy un dict"}})
        self.assertIn("error", resp)

    def test_unknown_method(self):
        resp = handle({"jsonrpc": "2.0", "id": 4, "method": "nope"})
        self.assertEqual(resp["error"]["code"], -32601)


if __name__ == "__main__":
    unittest.main()
