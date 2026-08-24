package prompt

import (
	"errors"
	"strings"
)

var (
	ErrPromptNil       = errors.New("prompt is nil")
	ErrReceiptRequired = errors.New("receipt input is required")
	ErrOptionsRequired = errors.New("at least one enrichment option is required")
)

// Prompt is a small builder for receipt enrichment prompts.
//
// The fields are intentionally private. Callers should configure the prompt
// through methods, not by mutating internal state directly. This keeps the
// prompt contract stable and prevents accidental corruption before rendering.
type Prompt struct {
	preamble     string
	receipt      any
	options      any
	rules        []string
	requirements []string
}

// NewPrompt creates a prompt builder with the default backend contract.
//
// Default rules and requirements are copied, not reused directly. Package-level
// slices are mutable in Go, so each Prompt must own its own copy to avoid state
// leaking between requests.
func NewPrompt() *Prompt {
	return &Prompt{
		preamble:     strings.TrimSpace(defaultPreamble),
		rules:        cloneStrings(defaultRules),
		requirements: cloneStrings(defaultResponseRequirements),
	}
}

// WithPreamble overrides the default preamble.
//
// Empty values are ignored so callers can pass optional configuration without
// accidentally producing a prompt with no role/task description.
func (p *Prompt) WithPreamble(preamble string) *Prompt {
	if p == nil {
		return p
	}

	preamble = strings.TrimSpace(preamble)
	if preamble != "" {
		p.preamble = preamble
	}

	return p
}

// WithReceipt sets the receipt payload that will be sent to the model.
//
// The prompt package does not enforce a concrete receipt type. The receipt
// processing layer owns the domain model, while this package only serializes
// whatever normalized input it receives.
func (p *Prompt) WithReceipt(receipt any) *Prompt {
	if p == nil {
		return p
	}

	p.receipt = receipt

	return p
}

// WithOptions sets selected enrichment option profiles.
//
// Options are intentionally accepted as any because the prompt renderer should
// not depend on the enrichment package. This avoids a circular dependency between
// prompt construction and enrichment domain types.
func (p *Prompt) WithOptions(options any) *Prompt {
	if p == nil {
		return p
	}

	p.options = options

	return p
}

// WithRules appends additional rules to the default contract.
//
// Rules are appended instead of replacing defaults because the default rules
// protect backend compatibility. Custom rules should narrow behavior, not remove
// core safety constraints like valid JSON and no invented product details.
func (p *Prompt) WithRules(rules []string) *Prompt {
	if p == nil {
		return p
	}

	p.rules = appendNonEmptyStrings(p.rules, rules)

	return p
}

// WithRequirements appends additional response requirements.
//
// Requirements describe the backend response contract. Keeping defaults in place
// ensures that custom requirements cannot accidentally remove required fields
// like receiptId, products, index, or rawName.
func (p *Prompt) WithRequirements(requirements []string) *Prompt {
	if p == nil {
		return p
	}

	p.requirements = appendNonEmptyStrings(p.requirements, requirements)

	return p
}

// Build validates the prompt input and renders the final text.
//
// Validation is done before rendering so prompt construction errors are reported
// close to the caller instead of failing later during an LLM request.
func (p *Prompt) Build() (string, error) {
	if p == nil {
		return "", ErrPromptNil
	}

	if err := p.validate(); err != nil {
		return "", err
	}

	return RenderPrompt(*p)
}

func (p *Prompt) validate() error {
	if p.receipt == nil {
		return ErrReceiptRequired
	}

	if p.options == nil {
		return ErrOptionsRequired
	}

	return nil
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)

	return cloned
}

func appendNonEmptyStrings(dst []string, src []string) []string {
	for _, value := range src {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		dst = append(dst, value)
	}

	return dst
}
