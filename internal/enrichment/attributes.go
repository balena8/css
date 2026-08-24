package enrichment

// AttributeKey is a controlled key used inside product enrichment attributes.
//
// LLM output must not be allowed to create arbitrary attribute names. Keeping
// attributes behind a small taxonomy makes downstream analytics, filtering and
// product matching predictable.
type AttributeKey string

const (
	AttributeCarbonated AttributeKey = "carbonated"
	AttributeVolume     AttributeKey = "volume"
	AttributeWeight     AttributeKey = "weight"
	AttributeFatPercent AttributeKey = "fat_percent"
	AttributeFlavor     AttributeKey = "flavor"
	AttributeUnknown    AttributeKey = "unknown"
)

// allAttributeKeys keeps the public list deterministic.
//
// Maps are useful for validation, but their iteration order is random. A stable
// slice is better for prompts, tests, logs and generated documentation.
var allAttributeKeys = []AttributeKey{
	AttributeCarbonated,
	AttributeVolume,
	AttributeWeight,
	AttributeFatPercent,
	AttributeFlavor,
	AttributeUnknown,
}

var supportedAttributeKeys = map[AttributeKey]struct{}{
	AttributeCarbonated: {},
	AttributeVolume:     {},
	AttributeWeight:     {},
	AttributeFatPercent: {},
	AttributeFlavor:     {},
	AttributeUnknown:    {},
}

func (k AttributeKey) String() string {
	return string(k)
}

// IsValid checks whether an attribute key belongs to the backend-supported taxonomy.
//
// This validation is especially important for LLM responses because models can
// easily invent useful-looking but unsupported keys.
func (k AttributeKey) IsValid() bool {
	_, ok := supportedAttributeKeys[k]
	return ok
}

// SupportedAttributeKeys returns a copy of the supported attribute list.
//
// Returning a copy prevents callers from mutating package-level taxonomy state.
func SupportedAttributeKeys() []AttributeKey {
	keys := make([]AttributeKey, len(allAttributeKeys))
	copy(keys, allAttributeKeys)

	return keys
}
