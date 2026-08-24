package factory

import (
	"testing"

	"github.com/blubaum/check-stateless-server/internal/config"
)

func TestNewCompletionProvider_OpenAI(t *testing.T) {
	keys := &config.LoadedKeys{OpenAI: "test-key"}

	provider, err := NewCompletionProvider("openai", "", "", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_Anthropic(t *testing.T) {
	keys := &config.LoadedKeys{Anthropic: "test-key"}

	provider, err := NewCompletionProvider("anthropic", "", "", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_Ollama(t *testing.T) {
	keys := &config.LoadedKeys{}

	provider, err := NewCompletionProvider("ollama", "", "", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_Unknown(t *testing.T) {
	keys := &config.LoadedKeys{}

	_, err := NewCompletionProvider("unknown", "", "", nil, keys)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewCompletionProvider_WithModel(t *testing.T) {
	keys := &config.LoadedKeys{OpenAI: "test-key"}

	provider, err := NewCompletionProvider("openai", "gpt-4", "", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider.ModelName() != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", provider.ModelName())
	}
}

func TestNewCompletionProvider_WithBaseURL(t *testing.T) {
	keys := &config.LoadedKeys{Anthropic: "test-key"}

	provider, err := NewCompletionProvider(
		"anthropic", "", "https://gateway.example.com/v1", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_OpenAI_BaseURLNoKey(
	t *testing.T,
) {
	keys := &config.LoadedKeys{}

	provider, err := NewCompletionProvider(
		"openai", "", "http://localhost:1234/v1", nil, keys)
	if err != nil {
		t.Fatalf("expected success with base URL and no key: %v",
			err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_OpenAI_NoKeyNoURL(
	t *testing.T,
) {
	keys := &config.LoadedKeys{}

	_, err := NewCompletionProvider(
		"openai", "", "", nil, keys)
	if err == nil {
		t.Fatal("expected error when no key and no base URL")
	}
}

func TestNewCompletionProvider_Gemini(t *testing.T) {
	keys := &config.LoadedKeys{Gemini: "test-key"}
	provider, err := NewCompletionProvider("gemini", "", "", nil, keys)
	if err != nil {
		t.Fatalf("NewCompletionProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewCompletionProvider_Gemini_NoKey(t *testing.T) {
	keys := &config.LoadedKeys{}
	_, err := NewCompletionProvider("gemini", "", "", nil, keys)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}
