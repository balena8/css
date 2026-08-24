package enrichment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/blubaum/check-stateless-server/internal/prompt"
	"github.com/blubaum/check-stateless-server/llm"
)

const (
	defaultReceiptEnrichmentSystemPrompt = "You are a receipt product enrichment assistant. Return only valid JSON. Do not wrap the response in markdown."
	defaultReceiptEnrichmentMaxTokens    = 4096
	defaultReceiptEnrichmentTemperature  = 0

	// Keep the retry budget small. LLM calls are usually expensive and slow,
	// so retries should protect only against transient provider failures,
	// not hide systemic prompt/validation issues.
	defaultReceiptEnrichmentMaxAttempts = 4
)

var (
	ErrCompletionProviderRequired = errors.New("completion provider is required")
	ErrReceiptRequired            = errors.New("receipt json is required")
	ErrNoEnrichmentOptions        = errors.New("at least one enrichment option is required")
	ErrNoValidEnrichmentOptions   = errors.New("at least one valid enrichment option is required")
)

// CompletionProvider is intentionally small.
// The enricher should not know whether the completion comes from OpenAI,
// Claude, a local model, or a mock implementation in tests.
type CompletionProvider interface {
	Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error)
}

type ReceiptLLMEnricherConfig struct {
	Options      map[string]OptionName
	SystemPrompt string
	TokenBudget  int

	// Logger is injected to keep this package usable in services with different
	// logging setups. When omitted, slog.Default is used.
	Logger *slog.Logger
}

// ReceiptLLMEnricher orchestrates prompt building, LLM execution, response
// parsing, validation and retry behavior.
//
// It does not own provider-specific concerns such as API keys, model routing,
// transport configuration or provider-level rate limiting.
type ReceiptLLMEnricher struct {
	completionProvider CompletionProvider
	options            map[string]OptionName
	systemPrompt       string
	tokenBudget        int
	logger             *slog.Logger
}

func NewReceiptLLMEnricher(
	completionProvider CompletionProvider,
	config ReceiptLLMEnricherConfig,
) (*ReceiptLLMEnricher, error) {
	// Fail fast during construction. A nil provider would otherwise panic
	// during request handling, where the root cause is harder to trace.
	if completionProvider == nil {
		return nil, ErrCompletionProviderRequired
	}

	if len(config.Options) == 0 {
		return nil, ErrNoEnrichmentOptions
	}

	systemPrompt := strings.TrimSpace(config.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultReceiptEnrichmentSystemPrompt
	}

	tokenBudget := config.TokenBudget
	if tokenBudget <= 0 {
		tokenBudget = defaultReceiptEnrichmentMaxTokens
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ReceiptLLMEnricher{
		completionProvider: completionProvider,

		// Copy user-provided options to prevent external mutation after the
		// enricher is constructed. This makes behavior stable across requests.
		options: cloneOptions(config.Options),

		systemPrompt: systemPrompt,
		tokenBudget:  tokenBudget,
		logger:       logger,
	}, nil
}

func (e *ReceiptLLMEnricher) EnrichReceipt(
	ctx context.Context,
	receiptJSON any,
) (*BackendProductEnrichmentResponse, error) {
	if e == nil {
		return nil, errors.New("receipt llm enricher is nil")
	}

	// Accepting nil context often leads to panics in lower-level libraries.
	// Falling back to context.Background keeps the method defensive while still
	// allowing callers to pass proper cancellation/deadline contexts.
	if ctx == nil {
		ctx = context.Background()
	}

	if receiptJSON == nil {
		return nil, ErrReceiptRequired
	}

	optionProfiles := CreateProfilesForChosenOptions(e.options)
	if len(optionProfiles) == 0 {
		return nil, ErrNoValidEnrichmentOptions
	}

	request, err := e.buildCompletionRequest(receiptJSON, optionProfiles)
	if err != nil {
		return nil, err
	}

	return e.enrichWithRetry(ctx, request)
}

func (e *ReceiptLLMEnricher) buildCompletionRequest(
	receiptJSON any,
	optionProfiles []OptionProfile,
) (llm.CompletionRequest, error) {
	promptText, err := prompt.NewPrompt().
		WithReceipt(receiptJSON).
		WithOptions(optionProfiles).
		Build()
	if err != nil {
		return llm.CompletionRequest{}, fmt.Errorf("build enrichment prompt: %w", err)
	}

	return llm.CompletionRequest{
		SystemPrompt: e.systemPrompt,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: promptText,
			},
		},
		MaxTokens: e.tokenBudget,

		// Enrichment is a structured extraction task, not creative generation.
		// Temperature 0 makes outputs more stable and easier to validate.
		Temperature: defaultReceiptEnrichmentTemperature,
	}, nil
}

func (e *ReceiptLLMEnricher) enrichWithRetry(
	ctx context.Context,
	req llm.CompletionRequest,
) (*BackendProductEnrichmentResponse, error) {
	var lastErr error

	for attempt := 1; attempt <= defaultReceiptEnrichmentMaxAttempts; attempt++ {
		response, err := e.enrichOnce(ctx, req, attempt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Only transient infrastructure/provider failures are retried.
		// Validation errors usually mean the prompt, parser or schema contract
		// is broken, so retrying would just repeat the same failure.
		if !isRetryableEnrichmentError(err) {
			return nil, err
		}

		if attempt == defaultReceiptEnrichmentMaxAttempts {
			break
		}

		delay := enrichmentRetryDelay(attempt)

		e.logger.WarnContext(
			ctx,
			"receipt enrichment failed with retryable error",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", defaultReceiptEnrichmentMaxAttempts),
			slog.Duration("retry_in", delay),
			slog.String("error", err.Error()),
		)

		if err := sleepWithContext(ctx, delay); err != nil {
			return nil, fmt.Errorf("receipt enrichment retry interrupted: %w", err)
		}
	}

	return nil, fmt.Errorf(
		"receipt enrichment failed after %d attempts: %w",
		defaultReceiptEnrichmentMaxAttempts,
		lastErr,
	)
}

func (e *ReceiptLLMEnricher) enrichOnce(
	ctx context.Context,
	req llm.CompletionRequest,
	attempt int,
) (*BackendProductEnrichmentResponse, error) {
	completionResponse, err := e.completionProvider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send enrichment prompt to llm: %w", err)
	}

	if completionResponse == nil {
		return nil, errors.New("llm completion response is nil")
	}

	rawContent := strings.TrimSpace(completionResponse.Content)

	// Log metadata, not the full prompt. Prompt/receipt payloads can contain
	// sensitive or noisy data. Raw response is logged only when parsing fails.
	e.logger.InfoContext(
		ctx,
		"receipt enrichment llm response received",
		slog.Int("attempt", attempt),
		slog.String("finish_reason", completionResponse.FinishReason),
		slog.Any("usage", completionResponse.Usage),
		slog.Int("raw_response_length", len(completionResponse.Content)),
	)

	if rawContent == "" {
		return nil, errors.New("llm completion response content is empty")
	}

	enrichmentResponse, err := ParseBackendProductEnrichmentResponse(rawContent)
	if err != nil {
		// Raw response is useful here because parse failures are usually caused
		// by malformed JSON, markdown wrapping, truncated output, or schema drift.
		e.logger.WarnContext(
			ctx,
			"failed to parse receipt enrichment llm response",
			slog.Int("attempt", attempt),
			slog.String("raw_response", rawContent),
			slog.String("error", err.Error()),
		)

		return nil, fmt.Errorf("parse enrichment llm response: %w", err)
	}

	return enrichmentResponse, nil
}

func ParseBackendProductEnrichmentResponse(
	rawResponse string,
) (*BackendProductEnrichmentResponse, error) {
	cleanResponse := cleanJSONResponse(rawResponse)
	if cleanResponse == "" {
		return nil, errors.New("response json is empty")
	}

	var result BackendProductEnrichmentResponse
	if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
		return nil, fmt.Errorf("invalid json response: %w", err)
	}

	// Parsing only proves that JSON is syntactically valid.
	// Validate enforces the business contract expected by the backend.
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("invalid enrichment response: %w", err)
	}

	return &result, nil
}

func cleanJSONResponse(rawResponse string) string {
	cleanResponse := strings.TrimSpace(rawResponse)
	if cleanResponse == "" {
		return ""
	}

	// The system prompt asks for raw JSON, but some models still return fenced
	// markdown. Be tolerant at the boundary and strict after cleanup.
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	// Some providers prepend/append explanatory text despite instructions.
	// Extracting the outermost JSON object keeps the parser resilient without
	// accepting completely invalid responses.
	startIndex := strings.Index(cleanResponse, "{")
	endIndex := strings.LastIndex(cleanResponse, "}")

	if startIndex >= 0 && endIndex > startIndex {
		cleanResponse = cleanResponse[startIndex : endIndex+1]
	}

	return strings.TrimSpace(cleanResponse)
}

func isRetryableEnrichmentError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())

	retryableParts := [...]string{
		"status 408",
		"status 409",
		"status 425",
		"status 429",
		"status 500",
		"status 502",
		"status 503",
		"status 504",
		"timeout",
		"deadline exceeded",
		"connection reset",
		"connection refused",
		"temporary",
		"unavailable",
		"high demand",
		"rate limit",
		"resource exhausted",
		"too many requests",

		// These can happen when the provider truncates or streams an incomplete
		// response during overload. They are worth retrying once or twice.
		"unexpected end of json input",
		"llm completion response content is empty",
		"llm completion response is nil",
	}

	for _, part := range retryableParts {
		if strings.Contains(message, part) {
			return true
		}
	}

	return false
}

func enrichmentRetryDelay(attempt int) time.Duration {
	delays := [...]time.Duration{
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}

	if attempt <= 0 {
		return delays[0]
	}

	index := attempt - 1
	if index >= len(delays) {
		return delays[len(delays)-1]
	}

	return delays[index]
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func cloneOptions(options map[string]OptionName) map[string]OptionName {
	cloned := make(map[string]OptionName, len(options))

	for key, value := range options {
		cloned[key] = value
	}

	return cloned
}
