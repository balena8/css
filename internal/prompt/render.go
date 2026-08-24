package prompt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

var compiledPromptTemplate = template.Must(
	template.New("receipt_product_enrichment_prompt").Parse(promptTemplate),
)

type templateData struct {
	Preamble         string
	RulesJSON        string
	RequirementsJSON string
	OptionsJSON      string
	ReceiptJSON      string
	ResponseSchema   string
}

// RenderPrompt renders the final LLM prompt from already validated prompt data.
//
// The renderer is intentionally separate from the builder. The builder owns
// mutation and validation, while the renderer owns serialization and template
// execution. This keeps both responsibilities easy to test independently.
func RenderPrompt(p Prompt) (string, error) {
	rulesJSON, err := toPrettyJSON(p.rules)
	if err != nil {
		return "", fmt.Errorf("marshal prompt rules: %w", err)
	}

	requirementsJSON, err := toPrettyJSON(p.requirements)
	if err != nil {
		return "", fmt.Errorf("marshal prompt requirements: %w", err)
	}

	optionsJSON, err := toPrettyJSON(p.options)
	if err != nil {
		return "", fmt.Errorf("marshal selected option profiles: %w", err)
	}

	receiptJSON, err := toPrettyJSON(p.receipt)
	if err != nil {
		return "", fmt.Errorf("marshal receipt input: %w", err)
	}

	responseSchema, err := toPrettyJSON(exampleResponseSchema())
	if err != nil {
		return "", fmt.Errorf("marshal response schema example: %w", err)
	}

	data := templateData{
		Preamble:         p.preamble,
		RulesJSON:        rulesJSON,
		RequirementsJSON: requirementsJSON,
		OptionsJSON:      optionsJSON,
		ReceiptJSON:      receiptJSON,
		ResponseSchema:   responseSchema,
	}

	var buf bytes.Buffer

	// The template is compiled once at package initialization. Prompt rendering
	// may happen on every enrichment request, so reparsing the template each time
	// would be unnecessary work.
	if err := compiledPromptTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}

	return strings.TrimSpace(buf.String()), nil
}

func toPrettyJSON(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}
