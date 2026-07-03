# Guía de instalación de Prompt Engineering Studio (PES)

Instrucciones paso a paso para instalar PES y todas sus dependencias en
Linux, macOS y Windows. Solo el **paso 1 y 2 son obligatorios**; todo lo
demás es opcional (PES funciona 100 % offline sin IA ni proveedores).

---

## Paso 1 — Instalar las dependencias obligatorias

Solo necesitas dos cosas: **Git** y **Go 1.24 o superior**.

### Linux (Debian/Ubuntu)

```bash
sudo apt update
sudo apt install -y git golang-go
go version   # debe mostrar go1.24 o superior
```

> Si tu distribución trae un Go antiguo (< 1.24), instala el oficial:
> ```bash
> curl -LO https://go.dev/dl/go1.24.7.linux-amd64.tar.gz
> sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.24.7.linux-amd64.tar.gz
> echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc && source ~/.bashrc
> ```

### macOS

```bash
# Con Homebrew (https://brew.sh):
brew install git go
go version
```

### Windows

1. Instala Git desde <https://git-scm.com/download/win> (siguiente-siguiente).
2. Instala Go desde <https://go.dev/dl/> (elige el instalador `.msi` de Windows).
3. Abre una terminal nueva (PowerShell) y comprueba: `go version`.

---

## Paso 2 — Descargar y compilar PES

```bash
# 1. Clonar el repositorio
git clone https://github.com/Vergil2599/Startup.git
cd Startup

# 2. Compilar (descarga las dependencias Go automáticamente; ~1 min la primera vez)
go build -o pes ./cmd/pes        # en Windows: go build -o pes.exe ./cmd/pes

# 3. Comprobar
./pes --version                  # en Windows: .\pes.exe --version
```

### (Recomendado) Instalar el binario en el PATH

```bash
# Linux / macOS:
sudo mv pes /usr/local/bin/
pes --version

# Windows (PowerShell como administrador):
#   mueve pes.exe a una carpeta que esté en tu PATH, por ejemplo:
mkdir "$env:LOCALAPPDATA\pes" -Force; move pes.exe "$env:LOCALAPPDATA\pes\"
#   y añade esa carpeta al PATH desde Configuración → Variables de entorno.
```

---

## Paso 3 — Primer uso (2 minutos)

```bash
# 1. Crear tu workspace (una carpeta normal con tus prompts como archivos .md)
pes init -w ~/mis-prompts        # instala 4 plantillas de inicio

# 2. Abrir la interfaz gráfica
pes ui -w ~/mis-prompts
# → abre http://127.0.0.1:8787 en tu navegador
```

En la UI puedes: crear prompts por bloques, ver la **puntuación de calidad en
vivo**, gestionar variables `{x}`, versionar (historial), comparar, componer
capas y exportar a md/txt/json/yaml/html/pdf/portapapeles.

Desde la terminal, lo mismo:

```bash
pes new "Mi primer prompt" -w ~/mis-prompts --tags demo
pes list -w ~/mis-prompts
pes validate <id> -w ~/mis-prompts
pes export <id> -w ~/mis-prompts -f md
```

> Consejo: si trabajas siempre en el mismo workspace, entra en la carpeta
> (`cd ~/mis-prompts`) y omite `-w` en todos los comandos.

---

## Paso 4 (opcional) — Conectar modelos LLM locales

Para usar el **simulador** y el **benchmark** necesitas al menos un proveedor.
La opción más sencilla y gratuita es Ollama.

### Opción A: Ollama (recomendada)

```bash
# 1. Instalar Ollama
#    Linux:
curl -fsSL https://ollama.com/install.sh | sh
#    macOS/Windows: descarga el instalador de https://ollama.com/download

# 2. Descargar un modelo (ejemplos)
ollama pull llama3.2        # ligero, ~2 GB
ollama pull qwen2.5-coder   # bueno para código

# 3. Decirle a PES dónde está: crea el archivo
#    ~/mis-prompts/.pes/providers.yaml con este contenido:
```

```yaml
providers:
  - {name: ollama, type: ollama, base_url: "http://localhost:11434", model: llama3.2}
```

```bash
# 4. Probar
pes run <id> -w ~/mis-prompts
pes bench <id> -w ~/mis-prompts -n 5     # benchmark: 5 repeticiones
```

### Opción B: LM Studio

1. Instala LM Studio desde <https://lmstudio.ai>, descarga un modelo y
   activa el servidor local (pestaña *Developer* → *Start Server*, puerto 1234).
2. Añade a `providers.yaml`:
   ```yaml
   - {name: lmstudio, type: openai, base_url: "http://localhost:1234", model: TU-MODELO}
   ```

### Opción C: APIs remotas (OpenRouter o compatible OpenAI)

```yaml
# providers.yaml — la API key NUNCA va en el archivo, solo el nombre de la
# variable de entorno donde la guardas:
  - {name: openrouter, type: openai, base_url: "https://openrouter.ai/api",
     model: "meta-llama/llama-3.1-70b-instruct", api_key_env: OPENROUTER_KEY}
```

```bash
export OPENROUTER_KEY="sk-or-…"   # en tu ~/.bashrc o equivalente
```

---

## Paso 5 (opcional) — IA de optimización (`pes-ai`)

El optimizer sugiere mejoras de tus prompts (redundancia, ambigüedad,
cortesía innecesaria…). Requiere **Python 3.10+**, sin ningún `pip install`
(solo usa la biblioteca estándar).

```bash
# 1. Comprobar Python
python3 --version        # Linux/macOS
python --version         # Windows (instálalo desde https://python.org si falta)

# 2. Nada más: si ejecutas pes desde el repositorio clonado, se autodetecta.
pes optimize <id> -w ~/mis-prompts

# 3. Si moviste el binario fuera del repo, indica la ruta una vez:
#    Linux/macOS (añádelo a tu ~/.bashrc):
export PES_AI_CMD="$HOME/Startup/sidecar/pes-ai.sh"
cat > ~/Startup/sidecar/pes-ai.sh << 'EOF'
#!/bin/sh
exec python3 "$(dirname "$0")/pes_ai/main.py" "$@"
EOF
chmod +x ~/Startup/sidecar/pes-ai.sh
```

Sin `pes-ai`, todo lo demás funciona igual: la IA es una ayuda, nunca una dependencia.

---

## Paso 6 (opcional) — Conectar PES a Claude Code / Cursor (MCP)

PES puede servir tu biblioteca como **servidor MCP**: tus agentes buscan,
leen, renderizan y validan tus prompts directamente.

### Claude Code

```bash
claude mcp add pes -- pes mcp -w ~/mis-prompts
```

### Cursor (u otro cliente MCP)

Añade a tu `mcp.json`:

```json
{
  "mcpServers": {
    "pes": { "command": "pes", "args": ["mcp", "-w", "/home/TU-USUARIO/mis-prompts"] }
  }
}
```

Herramientas disponibles para el agente: `search_prompts`, `list_prompts`,
`get_prompt`, `render_prompt` (con overrides de variables) y `validate_prompt`.

---

## Paso 7 (opcional) — Calidad en CI y anti-regresiones

```bash
# Gate de calidad: falla si algún prompt puntúa bajo (ideal en GitHub Actions)
pes lint -w . --min-score 70 --format github

# Anti-regresión: graba una respuesta de referencia y vigila la deriva
pes golden set <id> -P ollama
pes golden check <id> -P ollama --threshold 0.6   # exit 1 si diverge
```

Ejemplo de workflow de GitHub Actions:

```yaml
- uses: actions/setup-go@v5
  with: {go-version: "1.24"}
- run: go install github.com/Vergil2599/startup/pes/cmd/pes@latest
- run: pes lint -w ./prompts-repo --min-score 70 --format github
```

---

## Paso 8 (opcional) — Plugins

```bash
# Los plugins viven en <workspace>/.pes/plugins/<id>/ con su plugin.toml.
# Ejemplo incluido en el repo (Python, sin dependencias):
mkdir -p ~/mis-prompts/.pes/plugins
cp -r ~/Startup/plugins/example-python ~/mis-prompts/.pes/plugins/

pes plugin list -w ~/mis-prompts                       # descubierto, deshabilitado
pes plugin enable dev.pes.example-python -w ~/mis-prompts   # consentimiento explícito
```

---

## Copias de seguridad

```bash
pes backup -w ~/mis-prompts                       # tar.gz rotado (conserva 7)
pes backup -w ~/mis-prompts --passphrase "..."    # cifrado con age
```

Tus prompts son archivos Markdown normales: también puedes versionarlos con
git u hospedarlos en Obsidian — PES detecta las ediciones externas.

---

## Solución de problemas

| Síntoma | Solución |
|---|---|
| `no es un workspace de PES` | Ejecuta `pes init -w <carpeta>` primero |
| La búsqueda no encuentra algo que existe | `pes reindex -w <carpeta> --rebuild` (el índice es desechable, se reconstruye siempre) |
| `pes ui` dice "address already in use" | Usa otro puerto: `pes ui --addr 127.0.0.1:8890` |
| `pes run` → "no hay proveedores" | Crea `.pes/providers.yaml` (Paso 4) |
| `pes optimize` → "pes-ai no está instalado" | Instala Python 3 y/o define `PES_AI_CMD` (Paso 5) |
| Ollama no responde | Comprueba `ollama serve` y `curl http://localhost:11434` |
| `errores=N` al reindexar | El mensaje indica el archivo exacto; suele ser un .md editado a mano con el front-matter roto |

## Desinstalar

PES no toca el registro ni servicios: borra el binario `pes` y, si quieres,
la carpeta de tu workspace. Nada más.
