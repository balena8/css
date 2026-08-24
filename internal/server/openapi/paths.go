package openapi

func buildHealthPath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Health check",
			Description: "Checks whether the HTTP server is running.",
			OperationID: operationGetHealth,
			Tags:        []string{tagSystem},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("Server is healthy.", schemaHealthResponse),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func buildOpenAPIPath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Get OpenAPI specification",
			Description: "Returns the generated OpenAPI v3 document for check-stateless-server.",
			OperationID: operationGetOpenAPI,
			Tags:        []string{tagSystem},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("Generated OpenAPI document.", schemaOpenAPISpec),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func buildParseReceiptPath() OpenAPIPath {
	return OpenAPIPath{
		Post: &OpenAPIOperation{
			Summary: "Parse a receipt",
			Description: "Submits one fiscal receipt QR code for processing. " +
				"Single receipt requests are routed to the single-receipt queue to keep latency low.",
			OperationID: operationParseReceipt,
			Tags:        []string{tagReceipts},
			RequestBody: jsonRequestBody(
				"Single receipt parsing request.",
				schemaParseReceiptRequest,
				true,
			),
			Responses: standardProcessingResponses(schemaProcessReceiptsResponse),
		},
	}
}

func buildParseReceiptsBatchPath() OpenAPIPath {
	return OpenAPIPath{
		Post: &OpenAPIOperation{
			Summary: "Parse multiple receipts",
			Description: "Submits multiple fiscal receipt QR codes for processing. " +
				"Batch requests are routed to the batch queue, and each receipt returns its own processing result.",
			OperationID: operationParseReceiptsBatch,
			Tags:        []string{tagReceipts},
			RequestBody: jsonRequestBody(
				"Batch receipt parsing request.",
				schemaParseReceiptBatchRequest,
				true,
			),
			Responses: standardProcessingResponses(schemaProcessReceiptsResponse),
		},
	}
}

func buildReceiptQueuesPath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Get receipt queue stats",
			Description: "Returns queue statistics for both single-receipt and batch-receipt queues.",
			OperationID: operationGetReceiptQueues,
			Tags:        []string{tagReceiptQueues},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("Queue statistics.", schemaReceiptQueuesStatsResponse),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func buildSingleReceiptQueuePath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Get single receipt queue stats",
			Description: "Returns queue statistics for the single-receipt queue.",
			OperationID: operationGetSingleReceiptQueue,
			Tags:        []string{tagReceiptQueues},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("Single receipt queue statistics.", schemaReceiptQueueStats),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func buildBatchReceiptQueuePath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Get batch receipt queue stats",
			Description: "Returns queue statistics for the batch-receipt queue.",
			OperationID: operationGetBatchReceiptQueue,
			Tags:        []string{tagReceiptQueues},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("Batch receipt queue statistics.", schemaReceiptQueueStats),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func buildReceiptUserQueueStatusPath() OpenAPIPath {
	return OpenAPIPath{
		Get: &OpenAPIOperation{
			Summary:     "Get user receipt queue status",
			Description: "Returns pending and active receipt jobs for a specific user across both queues.",
			OperationID: operationGetReceiptUserQueue,
			Tags:        []string{tagReceiptQueues},
			Parameters: []OpenAPIParameter{
				{
					Name:        "user_id",
					In:          "query",
					Description: "User identifier used when submitting receipt jobs.",
					Required:    true,
					Schema:      stringSchema("User identifier."),
				},
			},
			Responses: map[string]OpenAPIResponse{
				statusOK:               jsonResponse("User queue status.", schemaUserQueueStatusResponse),
				statusBadRequest:       jsonResponse("Missing or invalid user_id query parameter.", schemaErrorResponse),
				statusMethodNotAllowed: jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
			},
		},
	}
}

func standardProcessingResponses(successSchema string) map[string]OpenAPIResponse {
	return map[string]OpenAPIResponse{
		statusOK:                  jsonResponse("Receipt processing result.", successSchema),
		statusBadRequest:          jsonResponse("Request body is invalid or required fields are missing.", schemaErrorResponse),
		statusRequestTimeout:      jsonResponse("Request was cancelled before processing completed.", schemaErrorResponse),
		statusGatewayTimeout:      jsonResponse("Receipt processing timed out.", schemaErrorResponse),
		statusMethodNotAllowed:    jsonResponse("HTTP method is not allowed.", schemaErrorResponse),
		statusInternalServerError: jsonResponse("Unexpected server error while processing receipts.", schemaErrorResponse),
	}
}
