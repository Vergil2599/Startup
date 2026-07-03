// Package ai define el puerto de proveedores LLM y sus implementaciones Go
// nativas (OpenAI-compatible y Ollama). La IA es SIEMPRE opcional en PES:
// nada del core depende de este paquete para funcionar.
package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Request es una petición de completado.
type Request struct {
	Model       string
	Prompt      string
	System      string
	Temperature float64
	MaxTokens   int
}

// Response es la respuesta normalizada de cualquier proveedor.
type Response struct {
	Text      string
	TokensIn  int
	TokensOut int
	Latency   time.Duration
}

// Provider es el puerto que implementan todos los backends LLM
// (incluidos los aportados por plugins).
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// ProviderConfig es una entrada de .pes/providers.yaml.
type ProviderConfig struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`     // "openai" | "ollama"
	BaseURL   string `yaml:"base_url"` // p. ej. http://localhost:11434
	Model     string `yaml:"model"`    // modelo por defecto
	APIKeyEnv string `yaml:"api_key_env,omitempty"`
	TimeoutS  int    `yaml:"timeout_seconds,omitempty"`
}

// LoadConfigs lee la configuración de proveedores del workspace.
// Un archivo ausente devuelve lista vacía (no es un error: modo offline puro).
func LoadConfigs(wsRoot string) ([]ProviderConfig, error) {
	data, err := os.ReadFile(filepath.Join(wsRoot, ".pes", "providers.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var wrapper struct {
		Providers []ProviderConfig `yaml:"providers"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("providers.yaml: %w", err)
	}
	for i, c := range wrapper.Providers {
		if c.Name == "" || c.BaseURL == "" {
			return nil, fmt.Errorf("providers.yaml: entrada %d sin name o base_url", i)
		}
		if c.Type != "openai" && c.Type != "ollama" {
			return nil, fmt.Errorf("providers.yaml: tipo desconocido %q (openai|ollama)", c.Type)
		}
	}
	return wrapper.Providers, nil
}

// Build construye el proveedor concreto desde su configuración.
// La API key nunca se guarda en YAML: se lee de la variable de entorno indicada.
func Build(c ProviderConfig) (Provider, error) {
	timeout := 120 * time.Second
	if c.TimeoutS > 0 {
		timeout = time.Duration(c.TimeoutS) * time.Second
	}
	apiKey := ""
	if c.APIKeyEnv != "" {
		apiKey = os.Getenv(c.APIKeyEnv)
	}
	switch c.Type {
	case "openai":
		return NewOpenAICompat(c.Name, c.BaseURL, apiKey, c.Model, timeout), nil
	case "ollama":
		return NewOllama(c.Name, c.BaseURL, c.Model, timeout), nil
	default:
		return nil, fmt.Errorf("tipo de proveedor desconocido: %q", c.Type)
	}
}
