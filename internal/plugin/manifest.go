// Package plugin implementa el sistema de plugins v1 de PES: procesos
// externos que hablan JSON-RPC 2.0 por stdio (una línea = un mensaje JSON).
// Cualquier lenguaje puede implementar un plugin; el SDK Python de ejemplo
// vive en plugins/example-python. Seguridad: los plugins declaran capacidades
// en su manifiesto y SOLO se cargan los habilitados explícitamente por el
// usuario en .pes/plugins.yaml.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

// APIVersion es la versión del contrato de plugins que habla este host.
const APIVersion = "1"

// Puntos de extensión soportados en v1.
const (
	ExtLLMProvider    = "llm.provider"
	ExtValidationRule = "validation.rule"
	ExtExportCodec    = "export.codec"
	ExtOptimizer      = "optimizer"
)

var validExts = map[string]bool{
	ExtLLMProvider: true, ExtValidationRule: true, ExtExportCodec: true, ExtOptimizer: true,
}

// Manifest es el contenido de plugin.toml.
type Manifest struct {
	Plugin struct {
		ID      string   `toml:"id"`
		Name    string   `toml:"name"`
		Version string   `toml:"version"`
		API     string   `toml:"api"`
		Entry   []string `toml:"entry"` // comando + args, relativo al dir del plugin
	} `toml:"plugin"`
	Capabilities struct {
		FSRead  []string `toml:"fs_read"`
		FSWrite []string `toml:"fs_write"`
		Network []string `toml:"network"`
	} `toml:"capabilities"`
	Contributes struct {
		ExtensionPoints []string `toml:"extension_points"`
	} `toml:"contributes"`
}

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// LoadManifest lee y valida un plugin.toml.
func LoadManifest(path string) (*Manifest, error) {
	var m Manifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, fmt.Errorf("manifiesto inválido: %w", err)
	}
	if !idRe.MatchString(m.Plugin.ID) {
		return nil, fmt.Errorf("manifiesto: id inválido %q", m.Plugin.ID)
	}
	if len(m.Plugin.Entry) == 0 {
		return nil, fmt.Errorf("manifiesto %s: falta entry", m.Plugin.ID)
	}
	if m.Plugin.API != APIVersion {
		return nil, fmt.Errorf("plugin %s requiere API %q; este host habla %q",
			m.Plugin.ID, m.Plugin.API, APIVersion)
	}
	if len(m.Contributes.ExtensionPoints) == 0 {
		return nil, fmt.Errorf("plugin %s no contribuye ningún extension point", m.Plugin.ID)
	}
	for _, ep := range m.Contributes.ExtensionPoints {
		if !validExts[ep] {
			return nil, fmt.Errorf("plugin %s: extension point desconocido %q", m.Plugin.ID, ep)
		}
	}
	return &m, nil
}

// Discovered es un plugin encontrado en disco (aún no cargado).
type Discovered struct {
	Manifest *Manifest
	Dir      string
	Enabled  bool
	LoadErr  string // manifiesto ilegible/incompatible: se reporta, no se carga
}

// Discover busca plugins en <workspace>/.pes/plugins/*/plugin.toml y marca
// como Enabled los listados en .pes/plugins.yaml (consentimiento explícito).
func Discover(wsRoot string) ([]Discovered, error) {
	pluginsDir := filepath.Join(wsRoot, ".pes", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	enabled, err := loadEnabled(wsRoot)
	if err != nil {
		return nil, err
	}
	var out []Discovered
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(pluginsDir, e.Name())
		d := Discovered{Dir: dir}
		m, merr := LoadManifest(filepath.Join(dir, "plugin.toml"))
		if merr != nil {
			d.LoadErr = merr.Error()
			out = append(out, d)
			continue
		}
		d.Manifest = m
		d.Enabled = enabled[m.Plugin.ID]
		out = append(out, d)
	}
	return out, nil
}

// loadEnabled lee la lista de plugins consentidos por el usuario.
func loadEnabled(wsRoot string) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(wsRoot, ".pes", "plugins.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var wrapper struct {
		Enabled []string `yaml:"enabled"`
	}
	if err := yamlUnmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("plugins.yaml: %w", err)
	}
	out := map[string]bool{}
	for _, id := range wrapper.Enabled {
		out[id] = true
	}
	return out, nil
}

// Enable añade un plugin a la lista de consentidos.
func Enable(wsRoot, id string) error {
	enabled, err := loadEnabled(wsRoot)
	if err != nil {
		return err
	}
	enabled[id] = true
	return saveEnabled(wsRoot, enabled)
}

// Disable retira el consentimiento de un plugin.
func Disable(wsRoot, id string) error {
	enabled, err := loadEnabled(wsRoot)
	if err != nil {
		return err
	}
	delete(enabled, id)
	return saveEnabled(wsRoot, enabled)
}

func saveEnabled(wsRoot string, enabled map[string]bool) error {
	ids := make([]string, 0, len(enabled))
	for id := range enabled {
		ids = append(ids, id)
	}
	data, err := yamlMarshal(struct {
		Enabled []string `yaml:"enabled"`
	}{Enabled: ids})
	if err != nil {
		return err
	}
	path := filepath.Join(wsRoot, ".pes", "plugins.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
