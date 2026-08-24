package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
	ProviderGemini    = "gemini"
	ProviderOllama    = "ollama"
)

const (
	EnvAnthropicAPIKey = "ANTHROPIC_API_KEY"
	EnvOpenAIAPIKey    = "OPENAI_API_KEY"
	EnvGeminiAPIKey    = "GEMINI_API_KEY"
)

const (
	DefaultAnthropicKeyFile = ".anthropic-api-key"
	DefaultOpenAIKeyFile    = ".openai-api-key"
	DefaultGeminiKeyFile    = ".gemini-api-key"
)

// LoadedKeys contains provider credentials loaded during application startup.
//
// Anthropic is intentionally kept here as a reserved provider. The config layer
// may know how to load its key before the completion provider is implemented.
// The factory still decides whether the provider can actually be created.
type LoadedKeys struct {
	Anthropic string
	OpenAI    string
	Gemini    string
}

type APIKeyLoader struct {
	config APIKeyFilesConfig
}

func NewAPIKeyLoader(cfg APIKeyFilesConfig) *APIKeyLoader {
	return &APIKeyLoader{
		config: cfg,
	}
}

// LoadKeyForProvider loads a key for one provider.
//
// Ollama does not require an API key for the default local setup. If a custom
// Ollama gateway needs auth later, pass it through enrichment.headers.
func (l *APIKeyLoader) LoadKeyForProvider(provider string) (string, error) {
	switch normalizeProvider(provider) {
	case ProviderAnthropic:
		return l.LoadAnthropicKey()

	case ProviderOpenAI:
		return l.LoadOpenAIKey()

	case ProviderGemini:
		return l.LoadGeminiKey()

	case ProviderOllama:
		return "", nil

	default:
		return "", fmt.Errorf("unsupported llm provider: %s", provider)
	}
}

// LoadKeysForReceiptEnrichment loads only the key required by the selected
// receipt enrichment provider.
//
// This happens during startup so missing credentials fail before the server
// starts accepting receipt processing requests.
func (l *APIKeyLoader) LoadKeysForReceiptEnrichment(
	enrichment ReceiptEnrichmentConfig,
) (*LoadedKeys, error) {
	keys := &LoadedKeys{}

	if !enrichment.Enabled {
		return keys, nil
	}

	switch normalizeProvider(enrichment.Provider) {
	case ProviderAnthropic:
		key, err := l.LoadAnthropicKey()
		if err != nil {
			return nil, err
		}

		keys.Anthropic = key

	case ProviderOpenAI:
		key, err := l.LoadOpenAIKey()
		if err != nil {
			return nil, err
		}

		keys.OpenAI = key

	case ProviderGemini:
		key, err := l.LoadGeminiKey()
		if err != nil {
			return nil, err
		}

		keys.Gemini = key

	case ProviderOllama:
		return keys, nil

	default:
		return nil, fmt.Errorf("unsupported receipt enrichment provider: %s", enrichment.Provider)
	}

	return keys, nil
}

func (l *APIKeyLoader) LoadAnthropicKey() (string, error) {
	return l.loadKey(
		l.config.Anthropic,
		EnvAnthropicAPIKey,
		DefaultAnthropicKeyFile,
		"Anthropic",
	)
}

func (l *APIKeyLoader) LoadOpenAIKey() (string, error) {
	return l.loadKey(
		l.config.OpenAI,
		EnvOpenAIAPIKey,
		DefaultOpenAIKeyFile,
		"OpenAI",
	)
}

func (l *APIKeyLoader) LoadGeminiKey() (string, error) {
	return l.loadKey(
		l.config.Gemini,
		EnvGeminiAPIKey,
		DefaultGeminiKeyFile,
		"Gemini",
	)
}

func (l *APIKeyLoader) loadKey(
	configPath string,
	envVar string,
	defaultFile string,
	providerName string,
) (string, error) {
	if configPath != "" {
		path, err := expandKeyPath(configPath)
		if err != nil {
			return "", fmt.Errorf("expand %s API key path: %w", providerName, err)
		}

		return readKeyFile(path, providerName)
	}

	if key := strings.TrimSpace(os.Getenv(envVar)); key != "" {
		return key, nil
	}

	defaultPath, err := buildDefaultKeyPath(defaultFile)
	if err != nil {
		return "", fmt.Errorf("build default %s API key path: %w", providerName, err)
	}

	return readKeyFile(defaultPath, providerName)
}

func readKeyFile(path string, providerName string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf(
				"%s API key file not found: %s",
				providerName,
				path,
			)
		}

		return "", fmt.Errorf("read %s API key file %q: %w", providerName, path, err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("%s API key file is empty: %s", providerName, path)
	}

	return key, nil
}

func buildDefaultKeyPath(defaultFile string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, defaultFile), nil
}

func expandKeyPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(homeDir, path[2:]), nil
	}

	return path, nil
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
