# 1. Visión del producto

## 1.1 Declaración de visión

> **Prompt Engineering Studio (PES)** es el entorno de trabajo profesional para prompts:
> un IDE de escritorio, offline-first, modular y extensible, donde cualquier persona —
> del principiante al staff engineer — puede **crear, componer, validar, optimizar,
> probar, versionar y reutilizar** prompts para cualquier modelo de IA, local o remoto.

La analogía guía: **lo que VS Code es para el código, PES lo es para el prompt engineering.**
No es un editor de texto con extras; es una plataforma con un núcleo pequeño y estable,
donde casi toda la funcionalidad de alto nivel (integraciones, exportadores, validadores
adicionales, proveedores LLM) se implementa como plugins sobre APIs públicas.

## 1.2 El problema

Hoy los prompts viven dispersos en notas, gists, archivos sueltos, historiales de chat y
repositorios sin estructura. Consecuencias:

- **No hay reutilización**: cada prompt se reescribe desde cero.
- **No hay calidad medible**: nadie sabe si un prompt es ambiguo, contradictorio o incompleto
  hasta que el modelo falla.
- **No hay versionado**: se pierde el historial de por qué un prompt cambió y qué funcionaba antes.
- **No hay portabilidad**: un prompt escrito para Claude no se adapta fácilmente a un modelo local.
- **No hay evaluación comparativa**: probar el mismo prompt en 3 modelos es un proceso manual.

## 1.3 La solución

Un estudio local donde el prompt es un **artefacto estructurado de primera clase**
(bloques tipados: Role, Objective, Context, Constraints, Rules, Workflow, Input, Output,
Examples, Failure Conditions, Success Criteria, Notes…), no un blob de texto. Sobre esa
estructura se construyen:

| Capacidad | Qué aporta |
|---|---|
| **Prompt Builder** | Editor visual por bloques activables/desactivables |
| **Plantillas con herencia** | Reutilización por categorías (Software, DevOps, Godot, SQL…) |
| **Variables** | `{{project_name}}`, `{{llm}}`… globales o por proyecto |
| **Composer** | Combinar prompts base + capas (estándares, reglas de salida) |
| **Biblioteca** | Carpetas, tags, favoritos, búsqueda instantánea (FTS local) |
| **Historial** | Versiones, diffs, autor, comentarios, restauración |
| **Validador** | Puntuación de calidad determinista, sin IA, 100 % offline |
| **Optimizer (opcional)** | Mejora asistida por LLM local — desactivable |
| **Simulador + Benchmark** | Ejecutar contra N modelos, comparar tiempo/tokens/costo/calidad |
| **Import/Export** | MD, JSON, YAML, TXT, HTML, PDF, clipboard |

## 1.4 Principios de producto (no negociables)

1. **Offline-first**: toda función esencial trabaja sin red. La IA es una ayuda, nunca una dependencia.
2. **Datos locales por defecto**: sin telemetría obligatoria; archivos legibles por humanos y git-friendly.
3. **Modular y extensible**: núcleo mínimo + sistema de plugins con API estable y versionada.
4. **Rápido**: abrir miles de prompts sin degradación; búsqueda < 50 ms; arranque < 2 s.
5. **Progresivo**: usable por principiantes en 5 minutos (plantillas + builder), potente para
   expertos (composer, variables, benchmark, atajos, plugins).
6. **Agnóstico de modelo**: Ollama, llama.cpp, LM Studio, OpenRouter, APIs compatibles OpenAI,
   Claude Code, OpenClaw, Gemini CLI… mediante una capa de proveedores conectable.

## 1.5 Usuarios objetivo y jobs-to-be-done

| Usuario | Job principal |
|---|---|
| Prompt Engineer | Iterar prompts con validación, versiones y benchmark |
| AI Engineer | Mantener bibliotecas de prompts por proyecto, integrarlas vía CLI/API |
| Desarrollador | Generar prompts de calidad para Claude Code / Cursor / Gemini CLI sin ser experto |
| Equipos de software | Estandarizar prompts (coding standards, output rules) y compartirlos por git |
| Investigadores | Comparar sistemáticamente respuestas entre modelos y guardar evidencia |
| Creadores de agentes / MCP | Componer system prompts largos por capas y validarlos |
| Usuarios de Obsidian | Prompts como Markdown con front-matter → interoperables con su vault |

## 1.6 Alcance del MVP vs. visión a largo plazo

- **MVP**: Builder por bloques, biblioteca con búsqueda, plantillas, variables, validador,
  export/import MD-JSON-YAML, historial local, temas claro/oscuro. Todo offline.
- **v1**: Composer, comparador, simulador con proveedores locales (Ollama/llama.cpp/LM Studio),
  Optimizer opcional (Python sidecar), export HTML/PDF, sistema de plugins estable.
- **v2**: Benchmark multi-modelo, integraciones (MCP, Obsidian, Git/GitHub, VS Code, Cursor,
  OpenClaw, Claude Code), sincronización opcional, marketplace de plugins/plantillas.

Detalle completo en [08-roadmap.md](08-roadmap.md).

## 1.7 Métricas de éxito

- **Time-to-first-prompt** (nuevo usuario → primer prompt exportado): < 5 minutos.
- **Búsqueda** sobre 10 000 prompts: < 50 ms p95.
- **Arranque en frío**: < 2 s en hardware modesto.
- **Cobertura offline**: 100 % de funciones núcleo sin red.
- **Extensibilidad**: un plugin "hello world" funcional en < 30 minutos siguiendo la guía.

## 1.8 Lo que PES no es

- No es un chat con LLM (el simulador ejecuta prompts, no mantiene conversaciones largas).
- No es un servicio SaaS ni requiere cuenta.
- No es un fine-tuning tool ni un orquestador de agentes (pero se integra con ellos).
