package factory

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blubaum/check-stateless-server/internal/config"
	"github.com/blubaum/check-stateless-server/llm"
	"github.com/blubaum/check-stateless-server/llm/gemini"
	"github.com/blubaum/check-stateless-server/llm/ollama"
	"github.com/blubaum/check-stateless-server/llm/openai"
)

var (
	ErrAnthropicProviderNotImplemented = errors.New("anthropic completion provider is not implemented yet")
)

// NewCompletionProvider creates a completion provider based on configuration.
//
// The factory is the only place that should know which concrete provider package
// corresponds to a config provider value. The rest of the application depends
// only on llm.CompletionProvider.
func NewCompletionProvider(
	providerType string,
	model string,
	baseURL string,
	headers map[string]string,
	apiKeys *config.LoadedKeys,
) (llm.CompletionProvider, error) {
	if apiKeys == nil {
		apiKeys = &config.LoadedKeys{}
	}

	provider := normalizeProvider(providerType)

	switch provider {
	case config.ProviderOpenAI:
		return newOpenAICompletionProvider(model, baseURL, headers, apiKeys)

	case config.ProviderGemini:
		return newGeminiCompletionProvider(model, baseURL, headers, apiKeys)

	case config.ProviderOllama:
		return newOllamaCompletionProvider(model, baseURL, headers)

	case config.ProviderAnthropic:
		return nil, ErrAnthropicProviderNotImplemented

	default:
		return nil, fmt.Errorf("unknown completion provider: %s", providerType)
	}
}

func newOpenAICompletionProvider(
	model string,
	baseURL string,
	headers map[string]string,
	apiKeys *config.LoadedKeys,
) (llm.CompletionProvider, error) {
	apiKey := strings.TrimSpace(apiKeys.OpenAI)
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)

	// OpenAI can be used in two modes:
	//  1. official OpenAI API with an API key;
	//  2. OpenAI-compatible gateway with a custom base URL and optional headers.
	if apiKey == "" && baseURL == "" {
		return nil, fmt.Errorf("OpenAI API key or custom base URL is required")
	}

	opts := []openai.CompletionOption{}

	if model != "" {
		opts = append(opts, openai.WithCompletionModel(model))
	}

	if baseURL != "" {
		opts = append(opts, openai.WithCompletionBaseURL(baseURL))
	}

	if len(headers) > 0 {
		opts = append(opts, openai.WithCompletionHeaders(headers))
	}

	return openai.NewCompletionProvider(apiKey, opts...), nil
}

func newGeminiCompletionProvider(
	model string,
	baseURL string,
	headers map[string]string,
	apiKeys *config.LoadedKeys,
) (llm.CompletionProvider, error) {
	apiKey := strings.TrimSpace(apiKeys.Gemini)
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)

	if apiKey == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}

	opts := []gemini.CompletionOption{}

	if model != "" {
		opts = append(opts, gemini.WithCompletionModel(model))
	}

	if baseURL != "" {
		opts = append(opts, gemini.WithCompletionBaseURL(baseURL))
	}

	if len(headers) > 0 {
		opts = append(opts, gemini.WithCompletionHeaders(headers))
	}

	return gemini.NewCompletionProvider(apiKey, opts...), nil
}

func newOllamaCompletionProvider(
	model string,
	baseURL string,
	headers map[string]string,
) (llm.CompletionProvider, error) {
	baseURL = strings.TrimSpace(baseURL)
	model = strings.TrimSpace(model)

	opts := []ollama.CompletionOption{}

	if model != "" {
		opts = append(opts, ollama.WithCompletionModel(model))
	}

	if baseURL != "" {
		opts = append(opts, ollama.WithCompletionBaseURL(baseURL))
	}

	if len(headers) > 0 {
		opts = append(opts, ollama.WithCompletionHeaders(headers))
	}

	return ollama.NewCompletionProvider(opts...), nil
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}
