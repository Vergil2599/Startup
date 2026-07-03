// Package diff implementa el comparador de PES: diff estructural por bloques
// (añadidos/eliminados/modificados/conmutados) y diff textual por líneas (LCS).
package diff

import (
	"strings"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

// Structural compara dos prompts a nivel de bloques y devuelve estadísticas.
func Structural(a, b *domain.Prompt) domain.Diff {
	d := domain.Diff{
		WordsA: countWords(a), WordsB: countWords(b),
		TitleDiff: a.Title != b.Title,
	}
	inA := map[domain.BlockType]domain.Block{}
	for _, blk := range a.Blocks {
		inA[blk.Type] = blk
	}
	seen := map[domain.BlockType]bool{}
	for _, nb := range b.Blocks {
		seen[nb.Type] = true
		ob, ok := inA[nb.Type]
		switch {
		case !ok:
			d.Blocks = append(d.Blocks, domain.BlockDiff{Type: nb.Type, Kind: domain.DiffAdded})
		case ob.Content != nb.Content:
			d.Blocks = append(d.Blocks, domain.BlockDiff{Type: nb.Type, Kind: domain.DiffModified})
		case ob.Enabled != nb.Enabled:
			d.Blocks = append(d.Blocks, domain.BlockDiff{Type: nb.Type, Kind: domain.DiffToggled})
		}
	}
	for _, ob := range a.Blocks {
		if !seen[ob.Type] {
			d.Blocks = append(d.Blocks, domain.BlockDiff{Type: ob.Type, Kind: domain.DiffRemoved})
		}
	}
	d.Equal = len(d.Blocks) == 0 && !d.TitleDiff
	return d
}

// Op es una operación de diff textual.
type Op struct {
	Kind string // "eq", "add", "del"
	Line string
}

// Lines calcula el diff línea a línea entre dos textos (LCS clásico).
func Lines(a, b string) []Op {
	al := splitLines(a)
	bl := splitLines(b)
	// Tabla LCS.
	n, m := len(al), len(bl)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []Op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case al[i] == bl[j]:
			ops = append(ops, Op{"eq", al[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, Op{"del", al[i]})
			i++
		default:
			ops = append(ops, Op{"add", bl[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, Op{"del", al[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, Op{"add", bl[j]})
	}
	return ops
}

// Unified produce una representación tipo diff unificado (sin contexto colapsado).
func Unified(a, b string) string {
	var sb strings.Builder
	for _, op := range Lines(a, b) {
		switch op.Kind {
		case "eq":
			sb.WriteString("  ")
		case "add":
			sb.WriteString("+ ")
		case "del":
			sb.WriteString("- ")
		}
		sb.WriteString(op.Line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}

func countWords(p *domain.Prompt) int {
	n := 0
	for _, b := range p.Blocks {
		n += len(strings.Fields(b.Content))
	}
	return n
}
