package cli

import (
	"os"
	"path/filepath"

	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
)

// starterTemplates se instalan en `pes init` si templates/ está vacío.
// IDs fijos para que la herencia entre ellas sea estable entre workspaces.
var starterTemplates = map[string]string{
	"software-development/base-coding.md": `---
pes: 1
id: 01PESTPLBASECODING00000000
title: Base de desarrollo de software
category: software-development
variables:
  language: Go
blocks:
  - {type: role, enabled: true}
  - {type: objective, enabled: true}
  - {type: constraints, enabled: true}
  - {type: rules, enabled: true}
  - {type: output, enabled: true}
---

## @role
Actúa como ingeniero de software senior especializado en {{language}}, con criterio pragmático: priorizas corrección, legibilidad y simplicidad sobre ingeniosidad.

## @objective
Describe aquí el resultado concreto y verificable que esperas (evita verbos vagos como "mejorar" o "ayudar").

## @constraints
- No introduzcas dependencias nuevas sin justificarlas.
- No reescrituras completas: cambios mínimos y dirigidos.

## @rules
- Sigue las convenciones idiomáticas de {{language}}.
- Todo código nuevo lleva sus tests.
- Si algo es ambiguo, pregunta antes de asumir.

## @output
Indica el formato exacto de la respuesta (diff, archivo completo, lista de hallazgos…).
`,
	"software-development/code-review.md": `---
pes: 1
id: 01PESTPLCODEREVIEW00000000
title: Revisor de código
category: software-development
extends: 01PESTPLBASECODING00000000
blocks:
  - {type: objective, enabled: true}
  - {type: workflow, enabled: true}
  - {type: output, enabled: true}
---

## @objective
Revisar el código adjunto y detectar defectos de corrección, casos límite sin cubrir y problemas de mantenibilidad. Reporta como mínimo los 3 hallazgos más severos.

## @workflow
1. Lee el código completo antes de opinar.
2. Identifica defectos de corrección (prioridad máxima).
3. Después, señala problemas de diseño o legibilidad.
4. Para cada hallazgo, propone la corrección concreta.

## @output
Lista en Markdown ordenada por severidad: archivo:línea — descripción — corrección propuesta.
`,
	"game-development/godot.md": `---
pes: 1
id: 01PESTPLGODOT0000000000000
title: Desarrollo con Godot
category: game-development
variables:
  godot_version: "4.3"
blocks:
  - {type: role, enabled: true}
  - {type: objective, enabled: true}
  - {type: rules, enabled: true}
  - {type: output, enabled: true}
---

## @role
Actúa como desarrollador experto en Godot {{godot_version}} y GDScript, con experiencia publicando juegos 2D y 3D.

## @objective
Describe la mecánica o sistema concreto a implementar y cómo sabrás que funciona.

## @rules
- GDScript idiomático con tipado estático (': tipo' y '-> tipo').
- Usa señales en lugar de acoplamiento directo entre nodos.
- Nombra nodos y escenas en PascalCase, archivos en snake_case.
- Explica la estructura de nodos antes del código.

## @output
Estructura de la escena (árbol de nodos) + código GDScript completo por archivo.
`,
	"writing/documento-tecnico.md": `---
pes: 1
id: 01PESTPLTECHWRITING0000000
title: Documento técnico
category: writing
variables:
  audience: desarrolladores
blocks:
  - {type: role, enabled: true}
  - {type: objective, enabled: true}
  - {type: context, enabled: true}
  - {type: constraints, enabled: true}
  - {type: output, enabled: true}
---

## @role
Actúa como technical writer senior que escribe para {{audience}}, claro y directo, sin relleno.

## @objective
Redactar el documento descrito en el contexto, optimizado para lectura en diagonal (títulos informativos, párrafos cortos).

## @context
Pega aquí el material fuente: notas, código, decisiones.

## @constraints
- Sin marketing ni adjetivos vacíos.
- Toda afirmación técnica debe ser verificable con el material fuente.

## @output
Markdown con jerarquía de encabezados; máximo 3 niveles.
`,
}

// installStarterTemplates escribe las plantillas si el directorio está vacío.
func installStarterTemplates(root string) (int, error) {
	dir := filepath.Join(root, fsrepo.DirTemplates)
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	if len(entries) > 0 {
		return 0, nil // no pisar plantillas del usuario
	}
	n := 0
	for rel, content := range starterTemplates {
		if err := fsrepo.WriteFileAtomic(filepath.Join(dir, rel), []byte(content)); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
