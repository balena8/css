package receipt

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// fiscalReceiptPhrase is a lightweight domain marker for fiscal receipts.
	//
	// The external response does not expose a single stable normalized boolean
	// for this in the model we use, so we derive the value from the decoded
	// human-readable receipt text.
	fiscalReceiptPhrase = "Касовий чек"

	moneyScale    = 100
	quantityScale = 1000
	percentScale  = 1

	receiptXMLTimestampLayout = "20060102150405"
	receiptJSONTimeLayout     = "2006-01-02 15:04:05"
)

var ErrReceiptXMLRequired = errors.New("receipt xml is required")

// BuildReceiptJSON converts raw data from the external tax API into the internal
// normalized receipt model returned by our service.
//
// The function intentionally accepts already decoded receiptText and checkXML.
// Decoding belongs to the transport/API processing layer because that layer knows
// how the payload arrived: base64, UTF-8, Windows-1251, etc.
//
// Mapping stays here because this package owns the receipt domain model.
func BuildReceiptJSON(
	apiResult ReceiptResponse,
	receiptText string,
	checkXML string,
) (*ReceiptJSON, error) {
	checkXML = strings.TrimSpace(checkXML)
	if checkXML == "" {
		return nil, ErrReceiptXMLRequired
	}

	var receiptXML ReceiptXML
	if err := xml.Unmarshal([]byte(checkXML), &receiptXML); err != nil {
		return nil, fmt.Errorf("parse receipt XML: %w", err)
	}

	// The external XML uses short names and scaled string values.
	// ReceiptJSON exposes stable API-friendly names, so this is the only place
	// where external XML shape is translated into our public response model.
	result := &ReceiptJSON{
		FiscalNumber:      strings.TrimSpace(receiptXML.Data.FiscalNumber),
		LocalNumber:       strings.TrimSpace(receiptXML.Data.Content.End.LocalNumber),
		Total:             moneyToFloat(receiptXML.Data.Content.End.Total),
		DateTime:          formatDateTime(receiptXML.Data.Content.End.Timestamp),
		TaxNumber:         strings.TrimSpace(receiptXML.Data.TaxNumber),
		FactoryNumber:     strings.TrimSpace(receiptXML.Data.LocalNumber),
		ReceiptText:       receiptText,
		IsFiscal:          isFiscalReceipt(receiptText),
		Products:          buildProducts(receiptXML.Data.Content.Products),
		Discounts:         buildDiscounts(receiptXML.Data.Content.Discounts),
		Payments:          buildPayments(receiptXML.Data.Content.Payments),
		Taxes:             buildTaxes(receiptXML.Data.Content.End.Taxes),
		ControlNumber:     strings.TrimSpace(receiptXML.MAC.Text),
		RawXML:            checkXML,
		OriginalAPIResult: apiResult,
	}

	return result, nil
}

// buildProducts maps XML product rows to the normalized JSON model.
//
// Monetary values in the XML are integer-like strings in minor units.
// Example: "12345" means 123.45 UAH.
//
// Quantity is also scaled separately.
// Example: "1000" usually means 1.000 item/kg/unit.
func buildProducts(products []ReceiptProductXML) []ProductJSON {
	if len(products) == 0 {
		return nil
	}

	result := make([]ProductJSON, 0, len(products))

	for _, product := range products {
		item := ProductJSON{
			Number:      strings.TrimSpace(product.Number),
			Code:        strings.TrimSpace(product.Code),
			Barcode:     strings.TrimSpace(product.Barcode),
			Name:        strings.TrimSpace(product.Name),
			Total:       moneyToFloat(product.Total),
			TaxGroup:    strings.TrimSpace(product.TaxGroup),
			MeasureUnit: strings.TrimSpace(product.MeasureUnit),
		}

		// Quantity is optional in some receipt lines.
		// A pointer keeps the difference between "missing" and "present but zero".
		if strings.TrimSpace(product.Quantity) != "" {
			quantity := quantityToFloat(product.Quantity)
			item.Quantity = &quantity
		}

		// Price is optional for the same reason as quantity.
		// Returning nil is more accurate than returning 0 for absent data.
		if strings.TrimSpace(product.Price) != "" {
			price := moneyToFloat(product.Price)
			item.Price = &price
		}

		result = append(result, item)
	}

	return result
}

// buildDiscounts maps discount rows and preserves their relation to products.
//
// The XML stores affected product numbers inside nested NI elements. The public
// JSON response exposes only a flat list of product line numbers because clients
// do not need to know the internal XML wrapper shape.
func buildDiscounts(discounts []ReceiptDiscountXML) []DiscountJSON {
	if len(discounts) == 0 {
		return nil
	}

	result := make([]DiscountJSON, 0, len(discounts))

	for _, discount := range discounts {
		items := make([]string, 0, len(discount.Items))

		for _, item := range discount.Items {
			number := strings.TrimSpace(item.Number)
			if number == "" {
				continue
			}

			items = append(items, number)
		}

		result = append(result, DiscountJSON{
			Number: strings.TrimSpace(discount.Number),
			Total:  moneyToFloat(discount.Total),
			Type:   strings.TrimSpace(discount.Type),
			Tax:    strings.TrimSpace(discount.TaxGroup),
			Items:  items,
		})
	}

	return result
}

// buildPayments maps payment data from XML.
//
// Payment terminal/provider fields often contain inconsistent spacing or empty
// values, so user-facing strings are normalized before being returned.
func buildPayments(payments []ReceiptPaymentXML) []PaymentJSON {
	if len(payments) == 0 {
		return nil
	}

	result := make([]PaymentJSON, 0, len(payments))

	for _, payment := range payments {
		result = append(result, PaymentJSON{
			Number:            strings.TrimSpace(payment.Number),
			Name:              strings.TrimSpace(payment.Name),
			PaymentSystem:     strings.TrimSpace(payment.PaymentSystem),
			Terminal:          strings.TrimSpace(payment.Terminal),
			Card:              strings.TrimSpace(payment.Card),
			AuthorizationCode: strings.TrimSpace(payment.AuthorizationCode),
			Transaction:       strings.TrimSpace(payment.RRN),
			Total:             moneyToFloat(payment.Total),
		})
	}

	return result
}

// buildTaxes maps tax totals from the XML receipt summary.
//
// Tax percent is not divided by 100 because this API field is already returned
// as a regular percentage-like numeric value, unlike money fields.
func buildTaxes(taxes []ReceiptTaxXML) []TaxJSON {
	if len(taxes) == 0 {
		return nil
	}

	result := make([]TaxJSON, 0, len(taxes))

	for _, tax := range taxes {
		result = append(result, TaxJSON{
			Name:    strings.TrimSpace(tax.Name),
			Percent: percentToFloat(tax.Percent),
			Total:   moneyToFloat(tax.Total),
		})
	}

	return result
}

// isFiscalReceipt detects whether the decoded printable receipt text contains
// the fiscal receipt marker.
//
// This check is intentionally case-insensitive because the source text is meant
// for humans and can vary in casing depending on API output or decoding.
func isFiscalReceipt(receiptText string) bool {
	text := strings.ToLower(strings.TrimSpace(receiptText))
	if text == "" {
		return false
	}

	return strings.Contains(text, strings.ToLower(fiscalReceiptPhrase))
}

// moneyToFloat converts money from minor units to major units.
//
// Example:
// "12345" -> 123.45
func moneyToFloat(value string) float64 {
	return parseScaledFloat(value, moneyScale)
}

// quantityToFloat converts quantity from the API scale to a regular decimal.
//
// Examples:
// "1000" -> 1
// "1500" -> 1.5
func quantityToFloat(value string) float64 {
	return parseScaledFloat(value, quantityScale)
}

// percentToFloat is intentionally separate from money/quantity conversion.
//
// It documents that tax percent currently requires no additional scaling and
// gives us one place to change the behavior if the external API changes format.
func percentToFloat(value string) float64 {
	return parseScaledFloat(value, percentScale)
}

// parseScaledFloat is a tolerant parser for numeric fields stored as strings.
//
// External receipt payloads can contain optional, empty, or malformed numeric
// fields. Mapping should produce the best possible receipt instead of failing
// the whole response because one optional numeric field is broken.
func parseScaledFloat(value string, scale float64) float64 {
	value = strings.TrimSpace(value)
	if value == "" || scale == 0 {
		return 0
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}

	return number / scale
}

// formatDateTime converts the compact tax API timestamp into a readable format.
//
// Expected input:
// 20260520153045
//
// Output:
// 2026-05-20 15:30:45
//
// If the value does not match the expected layout, the original source value is
// returned. This keeps debugging information available instead of hiding it.
func formatDateTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsedTime, err := time.Parse(receiptXMLTimestampLayout, value)
	if err != nil {
		return value
	}

	return parsedTime.Format(receiptJSONTimeLayout)
}
