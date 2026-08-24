package prompt

type BackendProductEnrichmentResponse struct {
	ReceiptID string                    `json:"receiptId"`
	Products  []ProductEnrichmentResult `json:"products"`
	Warnings  []string                  `json:"warnings,omitempty"`
}

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
