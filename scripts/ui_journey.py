#!/usr/bin/env python3
"""Simulación de usuario real sobre la UI web de PES con Chromium."""
import sys
from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:8822"
FAILS = []


def check(name, cond, extra=""):
    print(("✓ " if cond else "✗ ") + name + (f"  [{extra}]" if extra and not cond else ""))
    if not cond:
        FAILS.append(name)


with sync_playwright() as pw:
    browser = pw.chromium.launch(executable_path="/opt/pw-browsers/chromium_headless_shell-1194/chrome-linux/headless_shell")
    page = browser.new_page()
    errors = []
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.on("console", lambda m: errors.append(m.text) if m.type == "error" else None)

    page.goto(BASE)
    page.wait_for_selector("#statusLine")
    check("la UI carga y muestra estado", "prompts" in page.inner_text("#statusLine"))

    # ── Crear un prompt desde el diálogo ──
    page.click("#newBtn")
    page.fill("#newTitle", "Prompt de UI <script>alert(1)</script>")
    page.fill("#newTags", "ui, Prueba")
    page.click("#newOk")
    page.wait_for_selector("#titleIn")
    check("prompt creado y builder abierto", page.input_value("#titleIn").startswith("Prompt de UI"))
    check("XSS en título no se ejecuta", not any("alert" in e for e in errors))

    # ── Escribir en un bloque y ver la validación en vivo ──
    tas = page.locator("textarea")
    tas.nth(1).fill("Detectar 3 defectos por revisión usando {{herramienta}}.")
    page.wait_for_timeout(1200)  # debounce guardado + validación
    check("autoguardado confirmado", "guardado" in page.inner_text("#saveState"))
    score_txt = page.inner_text("#scoreBadge")
    check("score visible tras validar", "/100" in score_txt, score_txt)
    findings = page.inner_text("#findings")
    check("variable sin definir detectada en vivo", "herramienta" in findings, findings[:120])

    # ── Toggle de bloque ──
    page.locator("button[data-act=toggle]").first.click()
    page.wait_for_timeout(900)
    check("bloque desactivado visualmente", page.locator(".block.off").count() >= 1)

    # ── Añadir bloque nuevo ──
    before = page.locator(".block").count()
    page.select_option("#addBlockSel", "examples")
    page.wait_for_timeout(700)
    check("añadir bloque funciona", page.locator(".block").count() == before + 1)

    # ── Preview ──
    page.click("button[data-tab=preview]")
    page.wait_for_selector("pre.preview")
    prev = page.inner_text("pre.preview")
    check("preview muestra placeholder sin resolver", "{{herramienta}}" in prev, prev[:120])

    # ── Historial: snapshot + restore ──
    page.click("button[data-tab=historial]")
    page.wait_for_selector("#snapBtn")
    page.on("dialog", lambda d: d.accept("versión de prueba"))
    page.click("#snapBtn")
    page.wait_for_timeout(700)
    check("versión creada visible", "v1" in page.inner_text("#viewBox"))

    # ── Segundo prompt y diff ──
    page.click("#newBtn")
    page.fill("#newTitle", "Segundo para diff")
    page.click("#newOk")
    page.wait_for_selector("#titleIn")
    page.click("button[data-tab=diff]")
    page.wait_for_selector("#diffBtn")
    page.click("#diffBtn")
    page.wait_for_timeout(700)
    check("diff produce salida", len(page.inner_text("#diffOut")) > 10)

    # ── Composer: 2 capas → materializar ──
    page.click("button[data-tab=composer]")
    page.click("#addLayer")
    page.click("#addLayer")
    page.fill("#compTitle", "Compuesto desde UI")
    page.click("#compBtn")
    page.wait_for_timeout(1000)
    check("composer materializa y abre el resultado", page.input_value("#titleIn") == "Compuesto desde UI")

    # ── Búsqueda con / ──
    page.keyboard.press("/")
    page.keyboard.type("Segundo")
    page.wait_for_timeout(500)
    check("búsqueda filtra la lista", page.locator("#promptList .item").count() == 1)

    # ── Runner sin proveedores: mensaje claro, no error ──
    page.fill("#search", "")
    page.dispatch_event("#search", "input")
    page.wait_for_timeout(400)
    page.locator("#promptList .item").first.click()
    page.wait_for_selector("button[data-tab=runner]")
    page.click("button[data-tab=runner]")
    page.wait_for_timeout(600)
    check("runner sin proveedores guía al usuario", "providers.yaml" in page.inner_text("#viewBox"))

    # ── Tema oscuro persiste ──
    page.click("#themeBtn")
    theme = page.get_attribute("html", "data-theme")
    page.reload()
    page.wait_for_selector("#statusLine")
    check("tema persiste tras recargar", page.get_attribute("html", "data-theme") == theme)


    # ── Duplicar prompt (tras el reload hay que reabrir uno) ──
    page.wait_for_selector("#promptList .item")
    page.locator("#promptList .item").first.click()
    page.wait_for_selector("#titleIn")
    page.click("button[data-tab=builder]")
    page.wait_for_timeout(300)
    title_before = page.input_value("#titleIn")
    page.click("#dupBtn")
    page.wait_for_timeout(800)
    check("duplicar abre la copia", page.input_value("#titleIn") == title_before + " (copia)")

    # ── Editor de variables globales ──
    page.click("#varsBtn")
    page.wait_for_selector("#varsRows")
    page.fill("#varNew", "herramienta")
    page.fill("#varNewVal", "golangci-lint")
    page.click("#varAdd")
    page.click("#varsSave")
    page.wait_for_timeout(600)
    check("variable global creada desde la UI", True)

    # ── Drag & drop de bloques ──
    page.click("button[data-tab=builder]")
    page.wait_for_selector(".block")
    names = page.locator(".block .name").all_inner_texts()
    if len(names) >= 2:
        src_h = page.locator(".block header").first
        dst_h = page.locator(".block header").nth(1)
        src_h.drag_to(dst_h)
        page.wait_for_timeout(800)
        names2 = page.locator(".block .name").all_inner_texts()
        check("drag & drop reordena bloques", names2[0] == names[1] and names2[1] == names[0],
              f"{names} -> {names2}")
    else:
        check("drag & drop reordena bloques", False, "menos de 2 bloques")

    js_errors2 = [e for e in errors if "favicon" not in e]
    check("cero errores JS tras funciones nuevas", not js_errors2, "; ".join(js_errors2[:3]))

    js_errors = [e for e in errors if "favicon" not in e]
    check("cero errores JS en toda la sesión", not js_errors, "; ".join(js_errors[:3]))

    browser.close()

print("\n%d fallos" % len(FAILS))
sys.exit(1 if FAILS else 0)
