package openapi

// BuildOpenAPISpec builds the complete OpenAPI v3 document.
//
// The entry point is deliberately small. Paths and schemas are assembled by
// domain-specific builders below, which makes the API contract easier to grow,
// test, and review.
func BuildOpenAPISpec() OpenAPISpec {
	return OpenAPISpec{
		OpenAPI:    openAPIVersion,
		Info:       buildOpenAPIInfo(),
		Servers:    buildOpenAPIServers(),
		Paths:      buildOpenAPIPaths(),
		Components: buildOpenAPIComponents(),
	}
}

func buildOpenAPIInfo() OpenAPIInfo {
	return OpenAPIInfo{
		Title:       apiTitle,
		Description: apiDescription,
		Version:     apiVersion,
	}
}

func buildOpenAPIServers() []OpenAPIServer {
	return []OpenAPIServer{
		{
			URL:         apiV1URL,
			Description: apiV1Description,
		},
	}
}

func buildOpenAPIComponents() OpenAPIComponents {
	return OpenAPIComponents{
		Schemas: buildOpenAPISchemas(),
	}
}

func buildOpenAPIPaths() map[string]OpenAPIPath {
	return map[string]OpenAPIPath{
		pathHealth:                 buildHealthPath(),
		pathOpenAPI:                buildOpenAPIPath(),
		pathParseReceipt:           buildParseReceiptPath(),
		pathParseReceiptsBatch:     buildParseReceiptsBatchPath(),
		pathReceiptQueues:          buildReceiptQueuesPath(),
		pathSingleReceiptQueue:     buildSingleReceiptQueuePath(),
		pathBatchReceiptQueue:      buildBatchReceiptQueuePath(),
		pathReceiptUserQueueStatus: buildReceiptUserQueueStatusPath(),
	}
}
