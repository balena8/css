package openapi

const (
	openAPIVersion = "3.0.3"

	apiTitle       = "check-stateless-server"
	apiDescription = "Fiscal receipt processing API with asynchronous queues and optional LLM-based product enrichment."
	apiVersion     = "1.0.0"

	apiV1URL         = "/"
	apiV1Description = "Default API server"

	contentTypeJSON = "application/json"

	statusOK                  = "200"
	statusBadRequest          = "400"
	statusRequestTimeout      = "408"
	statusMethodNotAllowed    = "405"
	statusGatewayTimeout      = "504"
	statusInternalServerError = "500"

	tagSystem        = "System"
	tagReceipts      = "Receipts"
	tagReceiptQueues = "Receipt Queues"

	operationGetHealth             = "getHealth"
	operationGetOpenAPI            = "getOpenAPISpec"
	operationParseReceipt          = "parseReceipt"
	operationParseReceiptsBatch    = "parseReceiptsBatch"
	operationGetReceiptQueues      = "getReceiptQueues"
	operationGetSingleReceiptQueue = "getSingleReceiptQueue"
	operationGetBatchReceiptQueue  = "getBatchReceiptQueue"
	operationGetReceiptUserQueue   = "getReceiptUserQueueStatus"

	pathHealth                 = "/health"
	pathOpenAPI                = "/openapi.json"
	pathParseReceipt           = "/receipts/parse"
	pathParseReceiptsBatch     = "/receipts/parse/batch"
	pathReceiptQueues          = "/receipts/queues"
	pathSingleReceiptQueue     = "/receipts/queues/single"
	pathBatchReceiptQueue      = "/receipts/queues/batch"
	pathReceiptUserQueueStatus = "/receipts/queues/user"

	schemaHealthResponse           = "HealthResponse"
	schemaOpenAPISpec              = "OpenAPISpec"
	schemaParseReceiptRequest      = "ParseReceiptRequest"
	schemaParseReceiptBatchRequest = "ParseReceiptBatchRequest"
	schemaProcessReceiptsResponse  = "ProcessReceiptsResponse"
	schemaProcessReceiptResult     = "ProcessReceiptResult"
	schemaReceiptJSON              = "ReceiptJSON"
	schemaReceiptProduct           = "ReceiptProduct"
	schemaReceiptDiscount          = "ReceiptDiscount"
	schemaReceiptPayment           = "ReceiptPayment"
	schemaReceiptTax               = "ReceiptTax"
	schemaReceiptOriginalAPIResult = "ReceiptOriginalAPIResult"

	schemaEnrichmentResponse   = "EnrichmentResponse"
	schemaEnrichmentProduct    = "EnrichmentProduct"
	schemaEnrichmentAttributes = "EnrichmentAttributes"
	schemaEnrichmentMeasure    = "EnrichmentMeasure"
	schemaEnrichmentConfidence = "EnrichmentConfidence"

	schemaReceiptQueuesStatsResponse = "ReceiptQueuesStatsResponse"
	schemaReceiptQueueStats          = "ReceiptQueueStats"
	schemaReceiptPendingJobStats     = "ReceiptPendingJobStats"
	schemaReceiptActiveJobStats      = "ReceiptActiveJobStats"
	schemaUserQueueStatusResponse    = "UserQueueStatusResponse"
	schemaUserQueueSummary           = "UserQueueSummary"
	schemaUserQueueDetails           = "UserQueueDetails"

	schemaErrorResponse = "ErrorResponse"
	schemaErrorDetails  = "ErrorDetails"

	openAPIRefPrefix = "#/components/schemas/"
)

// OpenAPISpec is the root OpenAPI document returned by BuildOpenAPISpec.
//
// The model intentionally covers only the OpenAPI features currently used by
// this service. This keeps the specification strongly typed without replacing
// the whole structure with untyped map[string]any values.
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       OpenAPIInfo            `json:"info"`
	Servers    []OpenAPIServer        `json:"servers"`
	Paths      map[string]OpenAPIPath `json:"paths"`
	Components OpenAPIComponents      `json:"components"`
}

// OpenAPIInfo contains public metadata shown by OpenAPI documentation tools.
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// OpenAPIServer describes one API server entry.
//
// Keeping this explicit allows deployments to expose different base URLs later
// without changing route definitions.
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// OpenAPIPath represents supported operations for a single route.
//
// Only methods currently needed by the service are modeled. If the API grows,
// add fields explicitly instead of switching the whole structure to maps.
type OpenAPIPath struct {
	Get    *OpenAPIOperation `json:"get,omitempty"`
	Post   *OpenAPIOperation `json:"post,omitempty"`
	Put    *OpenAPIOperation `json:"put,omitempty"`
	Delete *OpenAPIOperation `json:"delete,omitempty"`
}

// OpenAPIOperation describes one HTTP operation in the API contract.
type OpenAPIOperation struct {
	Summary     string                     `json:"summary"`
	Description string                     `json:"description,omitempty"`
	OperationID string                     `json:"operationId"`
	Tags        []string                   `json:"tags,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
}

// OpenAPIParameter describes a path, query, header, or cookie parameter.
type OpenAPIParameter struct {
	Name        string        `json:"name"`
	In          string        `json:"in"`
	Description string        `json:"description,omitempty"`
	Required    bool          `json:"required"`
	Schema      OpenAPISchema `json:"schema"`
}

// OpenAPIRequestBody describes an operation request body.
type OpenAPIRequestBody struct {
	Description string                      `json:"description,omitempty"`
	Required    bool                        `json:"required"`
	Content     map[string]OpenAPIMediaType `json:"content"`
}

// OpenAPIResponse describes one operation response.
type OpenAPIResponse struct {
	Description string                      `json:"description"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

// OpenAPIMediaType describes the schema used for a specific content type.
type OpenAPIMediaType struct {
	Schema OpenAPISchema `json:"schema"`
}

// OpenAPISchema describes reusable request and response schemas.
//
// This type intentionally supports the subset of JSON Schema/OpenAPI keywords
// used by this service. If the contract grows, extend this type explicitly so
// unsupported schema behavior does not silently appear in the generated spec.
type OpenAPISchema struct {
	Type                 string                   `json:"type,omitempty"`
	Format               string                   `json:"format,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Properties           map[string]OpenAPISchema `json:"properties,omitempty"`
	Items                *OpenAPISchema           `json:"items,omitempty"`
	Required             []string                 `json:"required,omitempty"`
	Default              any                      `json:"default,omitempty"`
	Enum                 []string                 `json:"enum,omitempty"`
	MaxItems             *int                     `json:"maxItems,omitempty"`
	Ref                  string                   `json:"$ref,omitempty"`
	AdditionalProperties *OpenAPISchema           `json:"additionalProperties,omitempty"`
	Nullable             bool                     `json:"nullable,omitempty"`
}

// OpenAPIComponents contains reusable OpenAPI definitions.
type OpenAPIComponents struct {
	Schemas map[string]OpenAPISchema `json:"schemas"`
}
