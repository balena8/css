package openapi

func buildOpenAPISchemas() map[string]OpenAPISchema {
	schemas := map[string]OpenAPISchema{}

	addSystemSchemas(schemas)
	addReceiptRequestSchemas(schemas)
	addReceiptResponseSchemas(schemas)
	addEnrichmentSchemas(schemas)
	addQueueSchemas(schemas)
	addErrorSchemas(schemas)

	return schemas
}

func addSystemSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaHealthResponse] = objectSchema(
		"Response schema for health checks.",
		map[string]OpenAPISchema{
			"status": stringSchema("Health status, for example 'ok'."),
		},
		[]string{"status"},
	)

	schemas[schemaOpenAPISpec] = objectSchemaWithAdditionalProperties(
		"Generated OpenAPI v3 document.",
		&OpenAPISchema{},
	)
}

func addReceiptRequestSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaParseReceiptRequest] = objectSchema(
		"Request schema for parsing a single receipt QR code.",
		map[string]OpenAPISchema{
			"user_id": stringSchema("User identifier used for queue status tracking."),
			"qr_code": stringSchema("Fiscal receipt QR code URL from the tax cabinet."),
		},
		[]string{"user_id", "qr_code"},
	)

	schemas[schemaParseReceiptBatchRequest] = objectSchema(
		"Request schema for parsing multiple receipt QR codes.",
		map[string]OpenAPISchema{
			"user_id": stringSchema("User identifier used for queue status tracking."),
			"qr_codes": arraySchemaWithMaxItems(
				"Fiscal receipt QR code URLs from the tax cabinet.",
				stringSchema("Fiscal receipt QR code URL."),
				50,
			),
		},
		[]string{"user_id", "qr_codes"},
	)
}

func addReceiptResponseSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaProcessReceiptsResponse] = objectSchema(
		"Response returned after receipt processing finishes.",
		map[string]OpenAPISchema{
			"userId": stringSchema("User identifier from the original request."),
			"count":  integerSchema("Number of processed receipt items."),
			"results": arraySchema(
				"Per-receipt processing results.",
				schemaRef(schemaProcessReceiptResult),
			),
		},
		[]string{"userId", "count", "results"},
	)

	schemas[schemaProcessReceiptResult] = objectSchema(
		"Processing result for one receipt QR code.",
		map[string]OpenAPISchema{
			"status": enumStringSchema(
				"Receipt processing status.",
				[]string{"success", "failed"},
			),
			"message":     stringSchema("Human-readable processing message."),
			"receiptJson": nullableSchema(schemaRef(schemaReceiptJSON)),
			"enrichment":  nullableSchema(schemaRef(schemaEnrichmentResponse)),
		},
		[]string{"status", "message"},
	)

	schemas[schemaReceiptJSON] = objectSchema(
		"Normalized receipt model returned by the receipt processor.",
		map[string]OpenAPISchema{
			"fiscalNumber":  stringSchema("Fiscal number of the receipt."),
			"localNumber":   stringSchema("Local receipt number."),
			"total":         numberSchema("float", "Total receipt amount."),
			"dateTime":      stringSchema("Receipt date and time formatted by the backend."),
			"taxNumber":     stringSchema("Tax number from the receipt, if present."),
			"factoryNumber": stringSchema("Factory number from the receipt, if present."),
			"receiptText":   stringSchema("Decoded human-readable receipt text."),
			"isFiscal":      boolSchema("Whether the decoded receipt text looks like a fiscal receipt."),
			"products": arraySchema(
				"Products parsed from the receipt XML.",
				schemaRef(schemaReceiptProduct),
			),
			"discounts": arraySchema(
				"Discounts parsed from the receipt XML.",
				schemaRef(schemaReceiptDiscount),
			),
			"payments": arraySchema(
				"Payments parsed from the receipt XML.",
				schemaRef(schemaReceiptPayment),
			),
			"taxes": arraySchema(
				"Tax totals parsed from the receipt XML.",
				schemaRef(schemaReceiptTax),
			),
			"controlNumber":     stringSchema("Control number from the MAC/signature-like XML block."),
			"rawXml":            stringSchema("Normalized raw XML payload used for mapping."),
			"originalApiResult": schemaRef(schemaReceiptOriginalAPIResult),
		},
		[]string{
			"fiscalNumber",
			"localNumber",
			"total",
			"dateTime",
			"receiptText",
			"isFiscal",
			"products",
			"discounts",
			"payments",
			"taxes",
		},
	)

	schemas[schemaReceiptProduct] = objectSchema(
		"Product parsed from receipt XML.",
		map[string]OpenAPISchema{
			"number":      stringSchema("Sequential product number on the receipt."),
			"code":        stringSchema("Product code, if available."),
			"barcode":     stringSchema("Product barcode, if available."),
			"name":        stringSchema("Original product name from the receipt."),
			"quantity":    nullableSchema(numberSchema("float", "Product quantity.")),
			"price":       nullableSchema(numberSchema("float", "Single product price.")),
			"total":       numberSchema("float", "Total product price."),
			"taxGroup":    stringSchema("Tax group of the product, if available."),
			"measureUnit": stringSchema("Product measure unit, if available."),
		},
		[]string{"number", "name", "total"},
	)

	schemas[schemaReceiptDiscount] = objectSchema(
		"Discount parsed from receipt XML.",
		map[string]OpenAPISchema{
			"number": stringSchema("Discount line number."),
			"total":  numberSchema("float", "Discount amount."),
			"type":   stringSchema("Discount type, if available."),
			"tax":    stringSchema("Tax group related to the discount, if available."),
			"items": arraySchema(
				"Product line numbers affected by this discount.",
				stringSchema("Product line number."),
			),
		},
		[]string{"number", "total"},
	)

	schemas[schemaReceiptPayment] = objectSchema(
		"Payment parsed from receipt XML.",
		map[string]OpenAPISchema{
			"number":            stringSchema("Payment line number."),
			"name":              stringSchema("Payment method name."),
			"paymentSystem":     stringSchema("Payment system name, if available."),
			"terminal":          stringSchema("Payment terminal identifier, if available."),
			"card":              stringSchema("Masked card value, if available."),
			"authorizationCode": stringSchema("Payment authorization code, if available."),
			"transaction":       stringSchema("Payment transaction or RRN value, if available."),
			"total":             numberSchema("float", "Payment amount."),
		},
		[]string{"number", "total"},
	)

	schemas[schemaReceiptTax] = objectSchema(
		"Tax summary parsed from receipt XML.",
		map[string]OpenAPISchema{
			"name":    stringSchema("Tax name."),
			"percent": numberSchema("float", "Tax percent."),
			"total":   numberSchema("float", "Tax amount."),
		},
		[]string{"name", "percent", "total"},
	)

	schemas[schemaReceiptOriginalAPIResult] = objectSchema(
		"Raw response returned by the external tax API.",
		map[string]OpenAPISchema{
			"check":      stringSchema("Base64-encoded printable receipt payload."),
			"fn":         stringSchema("Fiscal number from the external API."),
			"name":       nullableSchema(stringSchema("Organization name, if provided.")),
			"addressGo":  nullableSchema(stringSchema("Organization address, if provided.")),
			"typeGo":     nullableSchema(stringSchema("Organization type, if provided.")),
			"tins":       nullableSchema(stringSchema("Tax identifier data, if provided.")),
			"xml":        boolSchema("Whether XML payload is available."),
			"sign":       boolSchema("Whether signature data is available."),
			"qr":         boolSchema("Whether QR data is available."),
			"resultCode": nullableSchema(stringSchema("External API result code, if provided.")),
			"resultText": stringSchema("External API result text."),
			"checkXml":   stringSchema("Base64-encoded receipt XML payload."),
			"checkP7s":   nullableSchema(stringSchema("Base64-encoded signature payload, if provided.")),
		},
		nil,
	)
}

func addEnrichmentSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaEnrichmentResponse] = objectSchema(
		"LLM enrichment response attached to a processed receipt.",
		map[string]OpenAPISchema{
			"receiptId": stringSchema("Stable receipt identifier used by enrichment."),
			"products": arraySchema(
				"Enriched products matched to original receipt product indexes.",
				schemaRef(schemaEnrichmentProduct),
			),
			"warnings": arraySchema(
				"Receipt-level enrichment warnings.",
				stringSchema("Warning message."),
			),
		},
		[]string{"receiptId", "products"},
	)

	schemas[schemaEnrichmentProduct] = objectSchema(
		"LLM enrichment result for one product.",
		map[string]OpenAPISchema{
			"index":       integerSchema("Original product index from the receipt input."),
			"rawName":     stringSchema("Original product name from the receipt."),
			"decodedName": nullableSchema(stringSchema("Decoded English product name.")),
			"category":    nullableSchema(stringSchema("Detected product category.")),
			"productType": nullableSchema(stringSchema("Detected product type.")),
			"brand":       nullableSchema(stringSchema("Detected product brand.")),
			"attributes":  nullableSchema(schemaRef(schemaEnrichmentAttributes)),
			"confidence":  nullableSchema(schemaRef(schemaEnrichmentConfidence)),
			"warnings": arraySchema(
				"Product-level enrichment warnings.",
				stringSchema("Warning message."),
			),
		},
		[]string{"index", "rawName"},
	)

	schemas[schemaEnrichmentAttributes] = OpenAPISchema{
		Type:        "object",
		Description: "Detected product attributes. Keys are controlled by the backend enrichment taxonomy.",
		Properties: map[string]OpenAPISchema{
			"carbonated":  boolSchema("Whether the product is carbonated."),
			"volume":      schemaRef(schemaEnrichmentMeasure),
			"weight":      schemaRef(schemaEnrichmentMeasure),
			"fat_percent": schemaRef(schemaEnrichmentMeasure),
			"flavor":      stringSchema("Detected product flavor."),
			"unknown":     stringSchema("Unknown or unsupported attribute value."),
		},
		AdditionalProperties: &OpenAPISchema{},
	}

	schemas[schemaEnrichmentMeasure] = objectSchema(
		"Numeric product measure with unit.",
		map[string]OpenAPISchema{
			"value": numberSchema("float", "Numeric measure value."),
			"unit":  stringSchema("Measure unit, for example ml, l, g, kg, or percent."),
		},
		[]string{"value", "unit"},
	)

	schemas[schemaEnrichmentConfidence] = OpenAPISchema{
		Type:        "object",
		Description: "Confidence scores for generated enrichment fields. Values are between 0 and 1.",
		Properties: map[string]OpenAPISchema{
			"decodedName": numberSchema("float", "Confidence score for decodedName."),
			"category":    numberSchema("float", "Confidence score for category."),
			"productType": numberSchema("float", "Confidence score for productType."),
			"brand":       numberSchema("float", "Confidence score for brand."),
			"attributes":  numberSchema("float", "Confidence score for attributes."),
		},
		AdditionalProperties: &OpenAPISchema{
			Type:   "number",
			Format: "float",
		},
	}
}

func addQueueSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaReceiptQueuesStatsResponse] = objectSchema(
		"Queue stats for both receipt queues.",
		map[string]OpenAPISchema{
			"single": schemaRef(schemaReceiptQueueStats),
			"batch":  schemaRef(schemaReceiptQueueStats),
		},
		[]string{"single", "batch"},
	)

	schemas[schemaReceiptQueueStats] = objectSchema(
		"Receipt queue statistics.",
		map[string]OpenAPISchema{
			"name":                stringSchema("Queue name."),
			"pending":             integerSchema("Number of jobs currently waiting in the queue channel."),
			"capacity":            integerSchema("Queue channel capacity."),
			"pending_jobs":        integerSchema("Number of jobs waiting to be processed."),
			"pending_users_count": integerSchema("Number of pending user/job entries."),
			"pending_users": arraySchema(
				"Pending jobs waiting in this queue.",
				schemaRef(schemaReceiptPendingJobStats),
			),
			"active_jobs":        integerSchema("Number of jobs currently processed by workers."),
			"active_users_count": integerSchema("Number of active user/job entries."),
			"active_users": arraySchema(
				"Active jobs currently processed by workers.",
				schemaRef(schemaReceiptActiveJobStats),
			),
		},
		[]string{
			"name",
			"pending",
			"capacity",
			"pending_jobs",
			"pending_users_count",
			"pending_users",
			"active_jobs",
			"active_users_count",
			"active_users",
		},
	)

	schemas[schemaReceiptPendingJobStats] = objectSchema(
		"Pending receipt job visible in queue stats.",
		map[string]OpenAPISchema{
			"job_id":        stringSchema("Receipt job identifier."),
			"user_id":       stringSchema("User identifier."),
			"receipt_count": integerSchema("Number of receipt QR codes in the job."),
			"created_at":    stringFormatSchema("date-time", "Time when the job was accepted by the queue."),
		},
		[]string{"job_id", "user_id", "receipt_count", "created_at"},
	)

	schemas[schemaReceiptActiveJobStats] = objectSchema(
		"Active receipt job currently processed by a worker.",
		map[string]OpenAPISchema{
			"job_id":        stringSchema("Receipt job identifier."),
			"user_id":       stringSchema("User identifier."),
			"receipt_count": integerSchema("Number of receipt QR codes in the job."),
			"worker_id":     integerSchema("Worker identifier inside the worker pool."),
			"started_at":    stringFormatSchema("date-time", "Time when the worker started processing the job."),
		},
		[]string{"job_id", "user_id", "receipt_count", "worker_id", "started_at"},
	)

	schemas[schemaUserQueueStatusResponse] = objectSchema(
		"Queue status for one user across both receipt queues.",
		map[string]OpenAPISchema{
			"user_id":    stringSchema("User identifier."),
			"is_queued":  boolSchema("Whether the user has at least one pending or active job."),
			"is_pending": boolSchema("Whether the user has at least one pending job."),
			"is_active":  boolSchema("Whether the user has at least one active job."),
			"summary":    schemaRef(schemaUserQueueSummary),
			"queues": arraySchema(
				"Detailed queue entries for this user.",
				schemaRef(schemaUserQueueDetails),
			),
		},
		[]string{"user_id", "is_queued", "is_pending", "is_active", "summary", "queues"},
	)

	schemas[schemaUserQueueSummary] = objectSchema(
		"Summary of pending and active user jobs.",
		map[string]OpenAPISchema{
			"pending_count": integerSchema("Number of pending jobs for the user."),
			"active_count":  integerSchema("Number of active jobs for the user."),
			"total_count":   integerSchema("Total number of pending and active jobs."),
		},
		[]string{"pending_count", "active_count", "total_count"},
	)

	schemas[schemaUserQueueDetails] = objectSchema(
		"One pending or active user job entry.",
		map[string]OpenAPISchema{
			"queue":         stringSchema("Queue name: single or batch."),
			"state":         enumStringSchema("Queue state.", []string{"pending", "active"}),
			"job_id":        stringSchema("Receipt job identifier."),
			"user_id":       stringSchema("User identifier."),
			"receipt_count": integerSchema("Number of receipt QR codes in the job."),
			"worker_id":     integerSchema("Worker identifier when the job is active."),
			"created_at":    stringFormatSchema("date-time", "Time when the pending job was created."),
			"started_at":    stringFormatSchema("date-time", "Time when active processing started."),
		},
		[]string{"queue", "state", "job_id", "user_id", "receipt_count"},
	)
}

func addErrorSchemas(schemas map[string]OpenAPISchema) {
	schemas[schemaErrorResponse] = objectSchema(
		"Error response schema.",
		map[string]OpenAPISchema{
			"error": schemaRef(schemaErrorDetails),
		},
		[]string{"error"},
	)

	schemas[schemaErrorDetails] = objectSchema(
		"Detailed error information.",
		map[string]OpenAPISchema{
			"code":    stringSchema("Stable machine-readable error code."),
			"message": stringSchema("Human-readable error message."),
			"details": objectSchemaWithAdditionalProperties(
				"Additional error details, if available.",
				&OpenAPISchema{},
			),
		},
		[]string{"code", "message"},
	)
}
