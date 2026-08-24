package prompt

// exampleResponseSchema returns a backend-compatible response shape example.
//
// This is intentionally not a strict JSON Schema. The real contract is enforced
// by backend validation after the LLM response is parsed. The example exists only
// to make the desired output structure concrete for the model.
//
// The example includes all possible enrichment fields as a superset. The rules
// and requirements still tell the model to omit fields for options that were not
// selected.
func exampleResponseSchema() map[string]any {
	return map[string]any{
		"receiptId": "receipt-id",
		"products": []map[string]any{
			{
				"index":       0,
				"rawName":     "МОРШИНСЬКА С/ГАЗ 1.5Л",
				"decodedName": "Morshinska sparkling water 1.5 l",
				"category":    "drinks",
				"productType": "sparkling_water",
				"brand":       "Morshinska",
				"attributes": map[string]any{
					"carbonated": true,
					"volume": map[string]any{
						"value": 1.5,
						"unit":  "l",
					},
				},
				"confidence": map[string]float64{
					"decodedName": 0.95,
					"category":    0.98,
					"productType": 0.94,
					"brand":       0.96,
					"attributes":  0.97,
				},
				"warnings": []string{},
			},
		},
		"warnings": []string{},
	}
}
