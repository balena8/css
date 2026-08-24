package config

import (
	"fmt"
	"strings"
)

// Validate checks whether loaded configuration is safe to use.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is required")
	}

	if c.Server.ListenAddress == "" {
		return fmt.Errorf("server listen_address is required")
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("server read_timeout must be greater than 0")
	}

	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("server write_timeout must be greater than 0")
	}

	if c.Server.IdleTimeout <= 0 {
		return fmt.Errorf("server idle_timeout must be greater than 0")
	}

	if c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("server tls cert_file is required when TLS is enabled")
		}

		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("server tls key_file is required when TLS is enabled")
		}
	}

	if err := c.validateReceiptService(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateReceiptService() error {
	if c.Receipt.TaxAPI.BaseURL == "" {
		return fmt.Errorf("receipt tax_api base_url is required")
	}

	if c.Receipt.TaxAPI.RequestTimeout <= 0 {
		return fmt.Errorf("receipt tax_api request_timeout must be greater than 0")
	}

	if c.Receipt.TaxAPI.CaptchaCode == "" {
		return fmt.Errorf("receipt tax_api captcha_code is required")
	}

	if c.Receipt.TaxAPI.ReceiptType == "" {
		return fmt.Errorf("receipt tax_api receipt_type is required")
	}

	if c.Receipt.Processing.Queue.SingleBuffer <= 0 {
		return fmt.Errorf("receipt processing queue single_buffer must be greater than 0")
	}

	if c.Receipt.Processing.Queue.BatchBuffer <= 0 {
		return fmt.Errorf("receipt processing queue batch_buffer must be greater than 0")
	}

	if c.Receipt.Processing.Workers.SingleWorkers <= 0 {
		return fmt.Errorf("receipt processing workers single_workers must be greater than 0")
	}

	if c.Receipt.Processing.Workers.BatchWorkers <= 0 {
		return fmt.Errorf("receipt processing workers batch_workers must be greater than 0")
	}

	if err := c.validateReceiptEnrichment(); err != nil {
		return err
	}

	return nil
}

func (c *Config) validateReceiptEnrichment() error {
	enrichment := &c.Receipt.Enrichment

	if !enrichment.Enabled {
		return nil
	}

	enrichment.Provider = normalizeProvider(enrichment.Provider)

	if enrichment.Provider == "" {
		return fmt.Errorf("receipt enrichment provider is required when enrichment is enabled")
	}

	if !isSupportedReceiptEnrichmentProvider(enrichment.Provider) {
		return fmt.Errorf("unsupported receipt enrichment provider: %s", enrichment.Provider)
	}

	if strings.TrimSpace(enrichment.Model) == "" {
		return fmt.Errorf("receipt enrichment model is required when enrichment is enabled")
	}

	// system_prompt is optional. When it is empty, the enrichment package uses
	// its own default prompt. This keeps prompt defaults close to prompt logic.
	enrichment.SystemPrompt = strings.TrimSpace(enrichment.SystemPrompt)

	if enrichment.TokenBudget <= 0 {
		return fmt.Errorf("receipt enrichment token_budget must be greater than 0")
	}

	if enrichment.Headers == nil {
		enrichment.Headers = map[string]string{}
	}

	return nil
}

func isSupportedReceiptEnrichmentProvider(provider string) bool {
	switch normalizeProvider(provider) {
	case ProviderOpenAI, ProviderGemini, ProviderOllama:
		return true

	case ProviderAnthropic:
		// The API key loader already supports Anthropic, but the completion
		// provider is not implemented yet. Keep config loading future-ready while
		// preventing runtime startup with an unsupported provider.
		return false

	default:
		return false
	}
}
