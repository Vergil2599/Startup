package importer

import (
	"strings"
	"testing"

	"github.com/Vergil2599/startup/pes/internal/domain"
)

func TestImportHostileInputsNeverPanic(t *testing.T) {
	hostile := []string{
		"{not json", "{\"title\": 12}", "---\nrota: [sin cerrar\n---\n",
		strings.Repeat("x", 1<<20), "\x00\x01\x02", "{}", "key: [1,2",
	}
	for i, in := range hostile {
		p, err := Import([]byte(in), "hostil")
		if err == nil && p == nil {
			t.Errorf("caso %d: ni prompt ni error", i)
		}
		if p != nil {
			if verr := p.Validate(); verr != nil {
				t.Errorf("caso %d: prompt importado inválido: %v", i, verr)
			}
		}
	}
	if _, err := Import([]byte("   \n  "), ""); err == nil {
		t.Fatal("archivo vacío debe dar error")
	}
}

func TestImportJSONAssignsIDWhenMissing(t *testing.T) {
	data := []byte(`{"title": "Sin ID", "blocks": [{"type": "objective", "enabled": true, "content": "x"}]}`)
	p, err := Import(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" {
		t.Fatal("debe asignarse un ID nuevo")
	}
	if b, ok := p.Block(domain.BlockObjective); !ok || b.Content != "x" {
		t.Fatalf("bloques: %+v", p.Blocks)
	}
}

func TestImportYAMLStructured(t *testing.T) {
	data := []byte("title: Desde YAML\nblocks:\n  - {type: role, enabled: true, content: rol}\n")
	p, err := Import(data, "")
	if err != nil || p.Title != "Desde YAML" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}

func TestImportPESMarkdownKeepsID(t *testing.T) {
	md := "---\npes: 1\nid: 01J8ZK7Q3W9X2Y4V5B6N7M8P9R\ntitle: PES nativo\nblocks:\n  - {type: role, enabled: true}\n---\n\n## @role\nhola\n"
	p, err := Import([]byte(md), "")
	if err != nil || string(p.ID) != "01J8ZK7Q3W9X2Y4V5B6N7M8P9R" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}
