package config

import (
	"fmt"
	"time"
)

const (
	defaultListenAddress = "0.0.0.0"
	defaultServerPort    = 8080

	defaultTaxReceiptAPIURL = "https://cabinet.tax.gov.ua/ws/api_public/rro/chkAllWeb"
	defaultCaptchaCode      = "0"
	defaultReceiptType      = "3"

	defaultReceiptEnrichmentProvider = ProviderGemini
	defaultReceiptEnrichmentModel    = "gemini-2.5-flash"
	defaultReceiptEnrichmentTokens   = 4096
)

// Config is the root application configuration.
//
// This package contains only passive configuration data loaded from YAML/env.
// Runtime objects such as queues, worker pools, processors, HTTP clients,
// loggers, and pipelines must be created in the application/bootstrap layer.
type Config struct {
	Server  ServerConfig         `yaml:"server"`
	APIKeys APIKeyFilesConfig    `yaml:"api_keys"`
	Receipt ReceiptServiceConfig `yaml:"receipt_service"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	Port          int           `yaml:"port"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`
	IdleTimeout   time.Duration `yaml:"idle_timeout"`
	TLS           TLSConfig     `yaml:"tls"`
	CORS          CORSConfig    `yaml:"cors"`
}

// TLSConfig contains HTTPS settings.
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// CORSConfig contains Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
}

// APIKeyFilesConfig contains paths to files with provider API keys.
//
// Empty values mean that the application may fallback to environment variables
// or provider-specific default files in the user's home directory.
type APIKeyFilesConfig struct {
	Anthropic string `yaml:"anthropic"`
	OpenAI    string `yaml:"openai"`
	Gemini    string `yaml:"gemini"`
}

// ReceiptServiceConfig contains everything needed to configure receipt processing.
type ReceiptServiceConfig struct {
	TaxAPI     TaxReceiptAPIConfig     `yaml:"tax_api"`
	Processing ReceiptProcessingConfig `yaml:"processing"`
	Enrichment ReceiptEnrichmentConfig `yaml:"enrichment"`
}

// TaxReceiptAPIConfig contains settings for the external tax receipt API.
type TaxReceiptAPIConfig struct {
	BaseURL        string        `yaml:"base_url"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	CaptchaCode    string        `yaml:"captcha_code"`
	ReceiptType    string        `yaml:"receipt_type"`
}

// ReceiptProcessingConfig contains queue and worker settings.
type ReceiptProcessingConfig struct {
	Queue   ReceiptQueueConfig  `yaml:"queue"`
	Workers ReceiptWorkerConfig `yaml:"workers"`
}

// ReceiptQueueConfig contains queue buffer sizes.
type ReceiptQueueConfig struct {
	SingleBuffer int `yaml:"single_buffer"`
	BatchBuffer  int `yaml:"batch_buffer"`
}

// ReceiptWorkerConfig contains worker pool settings.
type ReceiptWorkerConfig struct {
	SingleWorkers int `yaml:"single_workers"`
	BatchWorkers  int `yaml:"batch_workers"`
}

// ReceiptEnrichmentConfig contains LLM settings for receipt enrichment.
//
// This is not a RAG pipeline config. It describes the model used to enrich
// parsed receipt data with additional structured fields.
type ReceiptEnrichmentConfig struct {
	Enabled      bool              `yaml:"enabled"`
	Provider     string            `yaml:"provider"`
	Model        string            `yaml:"model"`
	BaseURL      string            `yaml:"base_url"`
	SystemPrompt string            `yaml:"system_prompt"`
	TokenBudget  int               `yaml:"token_budget"`
	Headers      map[string]string `yaml:"headers"`
}

// DefaultConfig returns production-safe defaults.
//
// These values are used as a base before YAML overrides are applied. Docker and
// local config files can override only the values that differ from defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddress: defaultListenAddress,
			Port:          defaultServerPort,
			ReadTimeout:   15 * time.Second,

			// Receipt processing may include queue waiting, external tax API calls
			// and optional LLM enrichment. The default write timeout must therefore
			// be long enough for synchronous HTTP handlers waiting on job results.
			WriteTimeout: 15 * time.Minute,

			IdleTimeout: 60 * time.Second,
			TLS: TLSConfig{
				Enabled: false,
			},
			CORS: CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{
					"GET",
					"POST",
					"OPTIONS",
				},
				AllowedHeaders: []string{
					"Authorization",
					"Content-Type",
				},
			},
		},

		APIKeys: APIKeyFilesConfig{},

		Receipt: ReceiptServiceConfig{
			TaxAPI: TaxReceiptAPIConfig{
				BaseURL:        defaultTaxReceiptAPIURL,
				RequestTimeout: 20 * time.Second,
				CaptchaCode:    defaultCaptchaCode,
				ReceiptType:    defaultReceiptType,
			},
			Processing: ReceiptProcessingConfig{
				Queue: ReceiptQueueConfig{
					SingleBuffer: 100,
					BatchBuffer:  100,
				},
				Workers: ReceiptWorkerConfig{
					SingleWorkers: 4,
					BatchWorkers:  2,
				},
			},
			Enrichment: ReceiptEnrichmentConfig{
				Enabled:     true,
				Provider:    defaultReceiptEnrichmentProvider,
				Model:       defaultReceiptEnrichmentModel,
				BaseURL:     "",
				TokenBudget: defaultReceiptEnrichmentTokens,

				// Empty system_prompt is valid. The enrichment package owns the
				// default prompt, so config should not duplicate prompt logic.
				SystemPrompt: "",

				Headers: map[string]string{},
			},
		},
	}
}

// ServerAddress returns host:port address for http.Server.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.ListenAddress, c.Server.Port)
}
