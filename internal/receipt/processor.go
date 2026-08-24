package receipt

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/blubaum/check-stateless-server/internal/enrichment"
	"golang.org/x/text/encoding/charmap"
)

const (
	// Tax cabinet is the only allowed source for receipt QR URLs.
	// Keeping it centralized prevents mismatches between validation, request
	// headers, and API URL construction.
	taxCabinetHost = "cabinet.tax.gov.ua"

	// Public receipt verification page path used inside QR codes.
	taxCabinetCheckPath = "/cashregs/check"

	// Public API endpoint used by the tax cabinet web page to fetch receipt data.
	taxReceiptAPIURL = "https://cabinet.tax.gov.ua/ws/api_public/rro/chkAllWeb"

	// The public tax API expects captcha to be present even when no real captcha
	// challenge is required by this flow.
	defaultCaptchaCode = "0"

	// Receipt type required by the tax API for fiscal receipt lookup.
	receiptTypeFiscal = "3"

	receiptSuccessMessage = "Receipt processed successfully"
)

var (
	requiredReceiptQRParams = []string{"date", "time", "id", "sm", "fn"}

	ErrReceiptProcessorNil   = errors.New("receipt processor is nil")
	ErrReceiptQRCodeRequired = errors.New("receipt qr code is required")
)

// ReceiptEnricher is the minimal dependency required by ReceiptProcessor.
//
// The receipt package depends on this interface instead of a concrete LLM
// implementation. This keeps receipt processing independent from providers,
// prompt rendering, model routing, API keys, and enrichment internals.
type ReceiptEnricher interface {
	EnrichReceipt(
		ctx context.Context,
		receiptJSON any,
	) (*enrichment.BackendProductEnrichmentResponse, error)
}

type ReceiptProcessor struct {
	client   *ReceiptAPIClient
	logger   *log.Logger
	enricher ReceiptEnricher
}

// ReceiptQRParams represents only the QR parameters required by the tax API.
//
// This type belongs to the receipt processing layer because it describes request
// parsing and API lookup, not the normalized receipt JSON model.
type ReceiptQRParams struct {
	ID     string
	Date   string
	Time   string
	Sum    string
	Fiscal string
}

// ReceiptEnrichmentInput is the compact input sent to the LLM.
//
// Do not send the whole ReceiptJSON to the model. It contains large and noisy
// fields such as receiptText, rawXml, originalApiResult.check, and checkXml.
// The enrichment model only needs a stable receiptId and product rows.
type ReceiptEnrichmentInput struct {
	ReceiptID string                       `json:"receiptId"`
	Products  []ReceiptProductEnrichmentIn `json:"products"`
}

// ReceiptProductEnrichmentIn is the compact product representation sent to LLM.
type ReceiptProductEnrichmentIn struct {
	Index    int      `json:"index"`
	RawName  string   `json:"rawName"`
	Code     string   `json:"code,omitempty"`
	Barcode  string   `json:"barcode,omitempty"`
	Number   string   `json:"number,omitempty"`
	Total    float64  `json:"total,omitempty"`
	Quantity *float64 `json:"quantity,omitempty"`
	Price    *float64 `json:"price,omitempty"`
}

// NewReceiptProcessor creates a processor with production defaults.
//
// The HTTP client is kept outside of processing logic so connections can be
// reused between requests. This avoids creating a new http.Client for every QR.
func NewReceiptProcessor() *ReceiptProcessor {
	return NewReceiptProcessorWithDependencies(
		NewReceiptAPIClient(nil),
		log.Default(),
		nil,
	)
}

// NewReceiptProcessorWithDependencies allows dependency injection without
// overengineering the package with unnecessary interfaces.
//
// Tests, CLI commands, background workers, and server setup code can provide
// their own client/logger/enricher while production code can use defaults.
func NewReceiptProcessorWithDependencies(
	client *ReceiptAPIClient,
	logger *log.Logger,
	enricher ReceiptEnricher,
) *ReceiptProcessor {
	if client == nil {
		client = NewReceiptAPIClient(nil)
	}

	if logger == nil {
		logger = log.Default()
	}

	return &ReceiptProcessor{
		client:   client,
		logger:   logger,
		enricher: enricher,
	}
}

// Process handles every QR code independently.
//
// One invalid receipt should not block the whole batch. This matters for
// user-facing APIs where clients may upload several receipts and still expect
// valid items to be processed even if one QR code is broken.
func (p *ReceiptProcessor) Process(
	ctx context.Context,
	qrCodes []string,
) ([]ProcessReceiptResult, error) {
	if p == nil {
		return nil, ErrReceiptProcessorNil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	results := make([]ProcessReceiptResult, 0, len(qrCodes))

	for _, qrCode := range qrCodes {
		results = append(results, p.processOne(ctx, qrCode))
	}

	return results, nil
}

// processOne converts internal processing errors into a stable response object.
//
// API clients always receive ProcessReceiptResult, so they do not need to handle
// different response shapes for successful and failed receipts.
func (p *ReceiptProcessor) processOne(
	ctx context.Context,
	qrCode string,
) ProcessReceiptResult {
	receiptJSON, err := p.buildReceiptResult(ctx, qrCode)
	if err != nil {
		p.logger.Printf("failed to process receipt: %v", err)

		return ProcessReceiptResult{
			Status:  ReceiptStatusFailed,
			Message: err.Error(),
		}
	}

	result := ProcessReceiptResult{
		Status:      ReceiptStatusSuccess,
		Message:     receiptSuccessMessage,
		ReceiptJSON: receiptJSON,
	}

	if p.enricher == nil {
		p.logReceiptSuccess(receiptJSON)
		return result
	}

	enrichmentInput, err := buildReceiptEnrichmentInput(receiptJSON)
	if err != nil {
		p.logger.Printf("failed to build receipt enrichment input: %v", err)

		result.Message = fmt.Sprintf(
			"%s, but enrichment input build failed: %v",
			receiptSuccessMessage,
			err,
		)

		return result
	}

	enrichmentResult, err := p.enricher.EnrichReceipt(ctx, enrichmentInput)
	if err != nil {
		p.logger.Printf("failed to enrich receipt: %v", err)

		result.Message = fmt.Sprintf(
			"%s, but enrichment failed: %v",
			receiptSuccessMessage,
			err,
		)

		return result
	}

	result.Enrichment = enrichmentResult

	p.logReceiptSuccess(receiptJSON)

	return result
}

// buildReceiptResult contains the processing pipeline for a single receipt:
//
//  1. validate QR URL
//  2. build tax API URL
//  3. fetch receipt data from the external API
//  4. decode text/XML payloads
//  5. normalize XML declaration
//  6. map raw data into ReceiptJSON
//
// Errors are wrapped at every step to make production debugging easier.
func (p *ReceiptProcessor) buildReceiptResult(
	ctx context.Context,
	qrCode string,
) (*ReceiptJSON, error) {
	if err := validateReceiptQRCodeURL(qrCode); err != nil {
		return nil, fmt.Errorf("validate receipt QR URL: %w", err)
	}

	apiURL, err := buildReceiptAPIURL(qrCode, defaultCaptchaCode)
	if err != nil {
		return nil, fmt.Errorf("build receipt API URL: %w", err)
	}

	// Do not log the full API URL. It contains receipt identifiers, fiscal
	// numbers, sums, and other payment-related values.
	p.logger.Printf("fetching receipt from tax API")

	apiResponse, err := p.client.Fetch(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch receipt from tax API: %w", err)
	}

	receiptText, err := decodeBase64ReceiptPayload(apiResponse.Check)
	if err != nil {
		return nil, fmt.Errorf("decode receipt text: %w", err)
	}

	receiptXML, err := decodeBase64ReceiptPayload(apiResponse.CheckXML)
	if err != nil {
		return nil, fmt.Errorf("decode receipt XML: %w", err)
	}

	// Some receipts are decoded from Windows-1251, but the XML declaration can
	// still contain the original encoding. Since the string is already UTF-8 at
	// this point, the declaration must be normalized before XML parsing.
	receiptXML = normalizeReceiptXMLDeclaration(receiptXML)

	receiptJSON, err := BuildReceiptJSON(*apiResponse, receiptText, receiptXML)
	if err != nil {
		return nil, fmt.Errorf("build receipt JSON: %w", err)
	}

	return receiptJSON, nil
}

// buildReceiptEnrichmentInput builds the minimal LLM input from ReceiptJSON.
//
// It also creates receiptId because ReceiptJSON does not have a dedicated
// receiptId field, while the enrichment response schema requires one.
func buildReceiptEnrichmentInput(receiptJSON *ReceiptJSON) (*ReceiptEnrichmentInput, error) {
	if receiptJSON == nil {
		return nil, errors.New("receipt json is required")
	}

	receiptID := buildReceiptID(receiptJSON)
	if receiptID == "" {
		return nil, errors.New("receipt id could not be built")
	}

	products := make([]ReceiptProductEnrichmentIn, 0, len(receiptJSON.Products))

	for index, product := range receiptJSON.Products {
		rawName := strings.TrimSpace(product.Name)
		if rawName == "" {
			continue
		}

		products = append(products, ReceiptProductEnrichmentIn{
			Index:    index,
			RawName:  rawName,
			Code:     strings.TrimSpace(product.Code),
			Barcode:  strings.TrimSpace(product.Barcode),
			Number:   strings.TrimSpace(product.Number),
			Total:    product.Total,
			Quantity: product.Quantity,
			Price:    product.Price,
		})
	}

	if len(products) == 0 {
		return nil, errors.New("receipt has no products for enrichment")
	}

	return &ReceiptEnrichmentInput{
		ReceiptID: receiptID,
		Products:  products,
	}, nil
}

// buildReceiptID creates a stable ID for enrichment.
//
// Preferred format:
// fiscalNumber-localNumber
//
// Example:
// 3000909908-696582
func buildReceiptID(receiptJSON *ReceiptJSON) string {
	if receiptJSON == nil {
		return ""
	}

	fiscalNumber := strings.TrimSpace(receiptJSON.FiscalNumber)
	localNumber := strings.TrimSpace(receiptJSON.LocalNumber)

	switch {
	case fiscalNumber != "" && localNumber != "":
		return fiscalNumber + "-" + localNumber
	case fiscalNumber != "":
		return fiscalNumber
	case localNumber != "":
		return localNumber
	default:
		return ""
	}
}

// validateReceiptQRCodeURL verifies that the QR URL belongs to the expected tax
// cabinet domain and contains all query parameters required for API lookup.
//
// This protects the service from accidentally processing arbitrary third-party
// URLs and gives clear validation errors before any external request is made.
func validateReceiptQRCodeURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ErrReceiptQRCodeRequired
	}

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("QR does not contain valid URL: %w", err)
	}

	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("invalid URL scheme: %s", parsedURL.Scheme)
	}

	if parsedURL.Host != taxCabinetHost {
		return fmt.Errorf("invalid URL host: %s", parsedURL.Host)
	}

	if !strings.HasPrefix(parsedURL.Path, taxCabinetCheckPath) {
		return fmt.Errorf("invalid URL path: %s", parsedURL.Path)
	}

	query := parsedURL.Query()

	for _, param := range requiredReceiptQRParams {
		if strings.TrimSpace(query.Get(param)) == "" {
			return fmt.Errorf("missing required query parameter: %s", param)
		}
	}

	return nil
}

// buildReceiptAPIURL converts a public QR URL into the tax API URL used for
// receipt lookup.
//
// The QR URL and the API endpoint use different parameter names and date formats,
// so this function is the single place where that translation happens.
func buildReceiptAPIURL(qrCodeURL string, captchaCode string) (string, error) {
	params, err := extractReceiptQRParams(qrCodeURL)
	if err != nil {
		return "", err
	}

	formattedDateTime, err := formatReceiptDateTime(params.Date, params.Time)
	if err != nil {
		return "", fmt.Errorf("format receipt date time: %w", err)
	}

	apiURL, err := url.Parse(taxReceiptAPIURL)
	if err != nil {
		return "", fmt.Errorf("parse receipt API URL: %w", err)
	}

	captchaCode = strings.TrimSpace(captchaCode)
	if captchaCode == "" {
		captchaCode = defaultCaptchaCode
	}

	apiQuery := url.Values{}
	apiQuery.Set("id", params.ID)
	apiQuery.Set("date", formattedDateTime)
	apiQuery.Set("type", receiptTypeFiscal)
	apiQuery.Set("captcha", captchaCode)
	apiQuery.Set("fn", params.Fiscal)
	apiQuery.Set("sm", params.Sum)

	apiURL.RawQuery = apiQuery.Encode()

	return apiURL.String(), nil
}

// extractReceiptQRParams maps short QR query parameters to domain-oriented names.
//
// The original QR parameter names are short:
//   - sm means receipt sum
//   - fn means fiscal/factory number
//
// Keeping this mapping isolated prevents unclear external names from spreading
// through the processing logic.
func extractReceiptQRParams(qrCodeURL string) (ReceiptQRParams, error) {
	parsedURL, err := url.Parse(qrCodeURL)
	if err != nil {
		return ReceiptQRParams{}, fmt.Errorf("parse QR code URL: %w", err)
	}

	query := parsedURL.Query()

	params := ReceiptQRParams{
		ID:     strings.TrimSpace(query.Get("id")),
		Date:   strings.TrimSpace(query.Get("date")),
		Time:   strings.TrimSpace(query.Get("time")),
		Sum:    strings.TrimSpace(query.Get("sm")),
		Fiscal: strings.TrimSpace(query.Get("fn")),
	}

	if err := params.Validate(); err != nil {
		return ReceiptQRParams{}, err
	}

	return params, nil
}

func (p ReceiptQRParams) Validate() error {
	if p.ID == "" {
		return errors.New("missing QR parameter: id")
	}

	if p.Date == "" {
		return errors.New("missing QR parameter: date")
	}

	if p.Time == "" {
		return errors.New("missing QR parameter: time")
	}

	if p.Sum == "" {
		return errors.New("missing QR parameter: sm")
	}

	if p.Fiscal == "" {
		return errors.New("missing QR parameter: fn")
	}

	return nil
}

// formatReceiptDateTime validates QR date/time values and formats them for the API.
//
// Expected QR format:
//   - date: YYYYMMDD
//   - time: HHMM or HHMMSS
//
// The tax API expects:
//   - YYYY-MM-DD HH:MM:SS
//
// time.Parse is used instead of manual string concatenation so impossible dates
// like month 13 or day 99 are rejected before calling the external API.
func formatReceiptDateTime(dateRaw string, timeRaw string) (string, error) {
	dateRaw = strings.TrimSpace(dateRaw)
	timeRaw = strings.TrimSpace(timeRaw)

	if len(dateRaw) != 8 {
		return "", errors.New("date must be in YYYYMMDD format")
	}

	switch len(timeRaw) {
	case 4:
		timeRaw += "00"
	case 6:
	default:
		return "", errors.New("time must be in HHMM or HHMMSS format")
	}

	rawDateTime := dateRaw + timeRaw

	parsedTime, err := time.Parse("20060102150405", rawDateTime)
	if err != nil {
		return "", fmt.Errorf("parse receipt datetime: %w", err)
	}

	return parsedTime.Format("2006-01-02 15:04:05"), nil
}

// decodeBase64ReceiptPayload decodes base64 payloads returned by the tax API.
//
// Most current responses are UTF-8, but older receipt payloads can be encoded
// with Windows-1251. We first check UTF-8 validity and fallback to Windows-1251
// only when necessary.
func decodeBase64ReceiptPayload(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	rawBytes, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}

	if utf8.Valid(rawBytes) {
		return string(rawBytes), nil
	}

	decodedBytes, err := charmap.Windows1251.NewDecoder().Bytes(rawBytes)
	if err != nil {
		return "", err
	}

	return string(decodedBytes), nil
}

// normalizeReceiptXMLDeclaration updates the XML declaration after the payload
// has already been decoded into UTF-8.
//
// Without this normalization, downstream XML parsers may treat an already-decoded
// UTF-8 string as Windows-1251 again if the original declaration is preserved.
func normalizeReceiptXMLDeclaration(value string) string {
	if value == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		`encoding="windows-1251"`, `encoding="UTF-8"`,
		`encoding='windows-1251'`, `encoding='UTF-8'`,
		`encoding="Windows-1251"`, `encoding="UTF-8"`,
		`encoding='Windows-1251'`, `encoding='UTF-8'`,
	)

	return replacer.Replace(value)
}

func (p *ReceiptProcessor) logReceiptSuccess(receiptJSON *ReceiptJSON) {
	if p == nil || p.logger == nil || receiptJSON == nil {
		return
	}

	p.logger.Printf(
		"successfully processed receipt fiscalNumber=%s localNumber=%s products=%d",
		receiptJSON.FiscalNumber,
		receiptJSON.LocalNumber,
		len(receiptJSON.Products),
	)
}
