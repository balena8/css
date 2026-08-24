package enrichment

import (
	"fmt"
	"strings"
)

const (
	confidenceFieldDecodedName = "decodedName"
	confidenceFieldCategory    = "category"
	confidenceFieldProductType = "productType"
	confidenceFieldBrand       = "brand"
	confidenceFieldAttributes  = "attributes"
)

var supportedConfidenceFields = map[string]struct{}{
	confidenceFieldDecodedName: {},
	confidenceFieldCategory:    {},
	confidenceFieldProductType: {},
	confidenceFieldBrand:       {},
	confidenceFieldAttributes:  {},
}

// BackendProductEnrichmentResponse is the backend-facing schema expected from LLM.
//
// This model belongs to the enrichment package because parsing and validation
// happen here. The prompt package may show an example response shape, but it
// should not own this contract.
type BackendProductEnrichmentResponse struct {
	ReceiptID string                    `json:"receiptId"`
	Products  []ProductEnrichmentResult `json:"products"`
	Warnings  []string                  `json:"warnings,omitempty"`
}

// ProductEnrichmentResult represents enrichment for one original receipt product.
//
// Index and RawName are required because they keep the LLM output linked to the
// original product row. All generated fields are optional because selected
// enrichment options can vary per request.
type ProductEnrichmentResult struct {
	Index       int                `json:"index"`
	RawName     string             `json:"rawName"`
	DecodedName *string            `json:"decodedName,omitempty"`
	Category    *string            `json:"category,omitempty"`
	ProductType *string            `json:"productType,omitempty"`
	Brand       *string            `json:"brand,omitempty"`
	Attributes  map[string]any     `json:"attributes,omitempty"`
	Confidence  map[string]float64 `json:"confidence,omitempty"`
	Warnings    []string           `json:"warnings,omitempty"`
}

// Validate enforces the backend contract after JSON parsing.
//
// json.Unmarshal only proves that the response is syntactically valid JSON.
// This method verifies that the parsed response is safe and meaningful for
// downstream backend logic.
func (r BackendProductEnrichmentResponse) Validate() error {
	if strings.TrimSpace(r.ReceiptID) == "" {
		return fmt.Errorf("receiptId is required")
	}

	if len(r.Products) == 0 {
		return fmt.Errorf("products are required")
	}

	for i := range r.Products {
		if err := r.Products[i].Validate(i); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks a single product enrichment result.
//
// The position argument is the array position in the LLM response, not the
// original receipt product index. It is used only to produce precise errors.
func (p ProductEnrichmentResult) Validate(position int) error {
	if p.Index < 0 {
		return fmt.Errorf("products[%d].index must be greater than or equal to zero", position)
	}

	if strings.TrimSpace(p.RawName) == "" {
		return fmt.Errorf("products[%d].rawName is required", position)
	}

	// Empty generated strings should be represented as null or omitted.
	// A pointer to an empty string is technically valid JSON, but it carries no
	// useful meaning and makes downstream logic harder to reason about.
	if err := validateOptionalText(position, "decodedName", p.DecodedName); err != nil {
		return err
	}

	if err := validateOptionalText(position, "brand", p.Brand); err != nil {
		return err
	}

	// Category and ProductType are pointers because enrichment options are
	// configurable. The field can be omitted when the caller did not request it,
	// but when present it must match the backend-supported taxonomy.
	if p.Category != nil {
		category := Category(strings.TrimSpace(*p.Category))
		if !category.IsValid() {
			return fmt.Errorf("products[%d].category has unsupported value %q", position, *p.Category)
		}
	}

	if p.ProductType != nil {
		productType := ProductType(strings.TrimSpace(*p.ProductType))
		if !productType.IsValid() {
			return fmt.Errorf("products[%d].productType has unsupported value %q", position, *p.ProductType)
		}
	}

	if err := validateAttributes(position, p.Attributes); err != nil {
		return err
	}

	if err := validateConfidence(position, p.Confidence); err != nil {
		return err
	}

	return nil
}

func validateOptionalText(position int, field string, value *string) error {
	if value == nil {
		return nil
	}

	if strings.TrimSpace(*value) == "" {
		return fmt.Errorf("products[%d].%s must not be empty when present", position, field)
	}

	return nil
}

func validateAttributes(position int, attributes map[string]any) error {
	// Attribute keys are intentionally limited. Letting the model invent new
	// keys would make analytics, filtering and downstream mapping unreliable.
	for key := range attributes {
		attributeKey := AttributeKey(strings.TrimSpace(key))
		if !attributeKey.IsValid() {
			return fmt.Errorf("products[%d].attributes has unsupported key %q", position, key)
		}
	}

	return nil
}

func validateConfidence(position int, confidence map[string]float64) error {
	// Confidence is a machine-readable score. Invalid ranges should be rejected
	// immediately instead of silently leaking bad data downstream.
	for field, value := range confidence {
		field = strings.TrimSpace(field)
		if field == "" {
			return fmt.Errorf("products[%d].confidence contains empty field name", position)
		}

		if _, ok := supportedConfidenceFields[field]; !ok {
			return fmt.Errorf("products[%d].confidence has unsupported field %q", position, field)
		}

		if value < 0 || value > 1 {
			return fmt.Errorf("products[%d].confidence[%q] must be between 0 and 1", position, field)
		}
	}

	return nil
}
