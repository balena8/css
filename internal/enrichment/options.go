package enrichment

import (
	"fmt"
	"sort"
	"strings"
)

// OptionName describes an enrichment capability requested by the caller.
//
// These values are used to build prompt profiles. The LLM receives only selected
// profiles, which helps keep the output focused and prevents it from returning
// fields that the backend/client did not ask for.
type OptionName string

const (
	OptionDecodeProducts    OptionName = "decode_products"
	OptionDetectBrands      OptionName = "detect_brands"
	OptionDetectAttributes  OptionName = "detect_attributes"
	OptionDetectCategories  OptionName = "detect_categories"
	OptionDetectProductType OptionName = "detect_product_types"
	OptionIncludeConfidence OptionName = "include_confidence"

	// Field-level option aliases are kept for compatibility with callers that
	// think in response fields instead of higher-level enrichment capabilities.
	OptionCategory    OptionName = "category"
	OptionProductType OptionName = "product_type"
	OptionAttributes  OptionName = "attributes"
	OptionDecodedName OptionName = "decoded_name"
)

// OptionProfile is the instruction block sent to the prompt renderer.
//
// The profile should describe not only what to produce, but also what must not
// be inferred. This is important for receipt enrichment because receipt names
// are short, abbreviated and often ambiguous.
type OptionProfile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

var optionOrder = map[OptionName]int{
	OptionDecodeProducts:    10,
	OptionDetectBrands:      20,
	OptionDetectAttributes:  30,
	OptionDetectCategories:  40,
	OptionDetectProductType: 50,
	OptionIncludeConfidence: 60,

	OptionDecodedName: 70,
	OptionCategory:    80,
	OptionProductType: 90,
	OptionAttributes:  100,
}

// CreateProfilesForChosenOptions converts selected options into deterministic
// prompt profiles.
//
// The input is a map, so its iteration order is random. Sorting prevents prompt
// text from changing between requests with the same options, which helps with
// reproducibility, testing and LLM output stability.
func CreateProfilesForChosenOptions(options map[string]OptionName) []OptionProfile {
	if len(options) == 0 {
		return nil
	}

	selectedOptions := uniqueOptionNames(options)
	sortOptionNames(selectedOptions)

	profiles := make([]OptionProfile, 0, len(selectedOptions))

	for _, option := range selectedOptions {
		profile, ok := profileForOption(option)
		if !ok {
			continue
		}

		profiles = append(profiles, profile)
	}

	return profiles
}

// DefaultReceiptEnrichmentOptions returns the standard enrichment profile set.
//
// A new map is returned on every call so callers can safely modify their local
// option set without changing global defaults.
func DefaultReceiptEnrichmentOptions() map[string]OptionName {
	return map[string]OptionName{
		string(OptionDecodeProducts):    OptionDecodeProducts,
		string(OptionDetectBrands):      OptionDetectBrands,
		string(OptionDetectAttributes):  OptionDetectAttributes,
		string(OptionDetectCategories):  OptionDetectCategories,
		string(OptionDetectProductType): OptionDetectProductType,
		string(OptionIncludeConfidence): OptionIncludeConfidence,
	}
}

func profileForOption(option OptionName) (OptionProfile, bool) {
	switch option {
	case OptionDecodeProducts, OptionDecodedName:
		return OptionProfile{
			Name: "Decode Products",
			Description: "Convert raw receipt product names, abbreviations, shortened words, transliterations, and unclear product labels into clear English product names. " +
				"The decoded name must stay faithful to the original receipt text. Do not add brand, volume, weight, flavor, carbonation, or other details unless they are directly present or strongly supported by the receipt text.",
			Example: `"Morshinska sparkling water 1.5 l" -> decodedName: "Morshinska sparkling water 1.5 l"`,
		}, true

	case OptionDetectCategories, OptionCategory:
		return OptionProfile{
			Name: "Detect Categories",
			Description: fmt.Sprintf(
				"Assign each receipt product to one of the allowed category values: %s. "+
					"The category must describe the broad product group, not the exact product type. "+
					"Use %s when the product text is too unclear to determine a category.",
				joinCategories(SupportedCategories()),
				CategoryUnknown,
			),
			Example: fmt.Sprintf(
				`"Morshinska sparkling water 1.5 l" -> category: "%s"`,
				CategoryDrinks,
			),
		}, true

	case OptionDetectBrands:
		return OptionProfile{
			Name: "Detect Brands",
			Description: "Detect the product brand only when it is explicitly present, clearly abbreviated, or strongly implied by the receipt product name. " +
				"Do not guess a brand from category or product type alone. If the brand is missing, unclear, or unreliable, return null.",
			Example: `"Morshinska sparkling water 1.5 l" -> brand: "Morshinska"`,
		}, true

	case OptionDetectAttributes, OptionAttributes:
		return OptionProfile{
			Name: "Detect Attributes",
			Description: fmt.Sprintf(
				"Extract only supported product attributes from the receipt product text. Use only these allowed attribute keys: %s. "+
					"Attribute values must be based only on information directly present or reliably inferable from the product name. "+
					"Use normalized units where possible, such as l, ml, g, kg, or %%. Do not create unsupported attributes.",
				joinAttributeKeys(SupportedAttributeKeys()),
			),
			Example: fmt.Sprintf(
				`"Morshinska sparkling water 1.5 l" -> attributes: { "%s": true, "%s": { "value": 1.5, "unit": "l" } }`,
				AttributeCarbonated,
				AttributeVolume,
			),
		}, true

	case OptionDetectProductType, OptionProductType:
		return OptionProfile{
			Name: "Detect Product Types",
			Description: fmt.Sprintf(
				"Identify the specific product type using only the allowed product type values: %s. "+
					"Product type must be more specific than category, but it must not include brand, volume, weight, flavor, or package size. "+
					"Use %s when the product type cannot be determined reliably.",
				joinProductTypes(SupportedProductTypes()),
				ProductTypeUnknown,
			),
			Example: fmt.Sprintf(
				`"Morshinska sparkling water 1.5 l" -> productType: "%s"`,
				ProductTypeSparklingWater,
			),
		}, true

	case OptionIncludeConfidence:
		return OptionProfile{
			Name: "Include Confidence",
			Description: "Include confidence scores for generated enrichment fields. Confidence values must be numbers from 0 to 1, where 1 means very certain and 0 means very uncertain. " +
				"Use higher confidence when the receipt product clearly supports the generated value. Use lower confidence when the product contains unclear abbreviations, incomplete names, spelling issues, or multiple possible interpretations.",
			Example: `"Morshinska sparkling water 1.5 l" -> confidence: { "decodedName": 0.95, "brand": 0.96, "category": 0.98, "productType": 0.94, "attributes": 0.97 }`,
		}, true

	default:
		return OptionProfile{}, false
	}
}

func uniqueOptionNames(options map[string]OptionName) []OptionName {
	seen := make(map[OptionName]struct{}, len(options))
	result := make([]OptionName, 0, len(options))

	for _, option := range options {
		if _, ok := seen[option]; ok {
			continue
		}

		seen[option] = struct{}{}
		result = append(result, option)
	}

	return result
}

func sortOptionNames(options []OptionName) {
	sort.SliceStable(options, func(i, j int) bool {
		leftOrder, leftKnown := optionOrder[options[i]]
		rightOrder, rightKnown := optionOrder[options[j]]

		if leftKnown && rightKnown {
			return leftOrder < rightOrder
		}

		if leftKnown {
			return true
		}

		if rightKnown {
			return false
		}

		return options[i] < options[j]
	})
}

func joinCategories(categories []Category) string {
	values := make([]string, 0, len(categories))

	for _, category := range categories {
		values = append(values, category.String())
	}

	return strings.Join(values, ", ")
}

func joinProductTypes(productTypes []ProductType) string {
	values := make([]string, 0, len(productTypes))

	for _, productType := range productTypes {
		values = append(values, productType.String())
	}

	return strings.Join(values, ", ")
}

func joinAttributeKeys(keys []AttributeKey) string {
	values := make([]string, 0, len(keys))

	for _, key := range keys {
		values = append(values, key.String())
	}

	return strings.Join(values, ", ")
}
