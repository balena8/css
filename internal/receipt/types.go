package receipt

import (
	"encoding/xml"

	"github.com/blubaum/check-stateless-server/internal/enrichment"
)

// ReceiptStatus is a controlled set of processing statuses returned to API clients.
//
// Using a dedicated type instead of plain string helps avoid inconsistent values
// like "ok", "error", "failed", "fail" across the codebase.
type ReceiptStatus string

const (
	ReceiptStatusSuccess ReceiptStatus = "success"
	ReceiptStatusFailed  ReceiptStatus = "failed"
)

// ReceiptXML describes the raw XML payload returned by the tax API.
//
// The external XML uses short element and attribute names such as RQ, DAT, C, FN,
// SM, TX, etc. Inside Go code we use domain-oriented field names to make the
// mapping layer easier to read, while XML tags preserve the original API format.
type ReceiptXML struct {
	XMLName xml.Name       `xml:"RQ"`
	Data    ReceiptXMLData `xml:"DAT"`
	MAC     ReceiptXMLMAC  `xml:"MAC"`
}

// ReceiptXMLData contains top-level receipt metadata from the DAT node.
//
// Most fields are stored as strings because the XML represents values as
// attributes. Conversion to numbers, dates, and normalized JSON names happens
// in the mapper, not in the XML transport model.
type ReceiptXMLData struct {
	DocumentID   string `xml:"DI,attr"`
	DateTime     string `xml:"DT,attr"`
	FiscalNumber string `xml:"FN,attr"`
	TaxNumber    string `xml:"TN,attr"`
	Version      string `xml:"V,attr"`
	LocalNumber  string `xml:"ZN,attr"`

	Content   ReceiptXMLContent `xml:"C"`
	Timestamp string            `xml:"TS"`
}

// ReceiptXMLContent represents the main receipt body.
//
// It contains printable text lines, product rows, discounts, payments, and final
// summary data. This structure mirrors the XML hierarchy but keeps Go field names
// readable for the rest of the package.
type ReceiptXMLContent struct {
	CashierName   string `xml:"AT_CN,attr"`
	CashierOffice string `xml:"AT_ON,attr"`
	CashierVAT    string `xml:"AT_V,attr"`
	ReceiptType   string `xml:"T,attr"`

	Lines     []ReceiptLineXML     `xml:"L"`
	Products  []ReceiptProductXML  `xml:"P"`
	Discounts []ReceiptDiscountXML `xml:"D"`
	Payments  []ReceiptPaymentXML  `xml:"M"`
	End       ReceiptEndXML        `xml:"E"`
}

// ReceiptLineXML represents a raw printable line from the receipt.
//
// These lines are useful when the client needs the human-readable receipt view,
// while product/payment/tax nodes are used for structured data.
type ReceiptLineXML struct {
	Number string `xml:"N,attr"`
	Text   string `xml:",chardata"`
}

// ReceiptProductXML represents a product row exactly as it appears in the XML.
//
// Money and quantity fields are strings here because the API stores them in
// scaled integer-like formats. For example, money usually needs division by 100,
// while quantity usually needs division by 1000.
type ReceiptProductXML struct {
	TotalLabel  string `xml:"AT_TL,attr"`
	MeasureUnit string `xml:"AT_TM,attr"`
	Code        string `xml:"C,attr"`
	Barcode     string `xml:"CD,attr"`
	Number      string `xml:"N,attr"`
	Name        string `xml:"NM,attr"`
	Price       string `xml:"PRC,attr"`
	Quantity    string `xml:"Q,attr"`
	Total       string `xml:"SM,attr"`
	TaxGroup    string `xml:"TX,attr"`
}

// ReceiptDiscountXML represents a discount row from the XML.
//
// Discounts can reference one or more product line numbers through nested NI
// elements. The mapper flattens those references into []string for the JSON model.
type ReceiptDiscountXML struct {
	TotalLabel string                   `xml:"AT_TL,attr"`
	Number     string                   `xml:"N,attr"`
	Total      string                   `xml:"SM,attr"`
	Rate       string                   `xml:"TR,attr"`
	TaxGroup   string                   `xml:"TX,attr"`
	Type       string                   `xml:"TY,attr"`
	Items      []ReceiptDiscountItemXML `xml:"NI"`
}

// ReceiptDiscountItemXML represents a product line reference inside a discount.
//
// The XML uses NI both as the element name and the attribute name, so the Go field
// is renamed to Number to make its meaning clear in code.
type ReceiptDiscountItemXML struct {
	Number string `xml:"NI,attr"`
}

// ReceiptPaymentXML represents a payment method row from the XML.
//
// Field names describe their normalized meaning where possible, while XML tags
// keep the original short attribute names used by the tax API.
type ReceiptPaymentXML struct {
	PaymentName       string `xml:"AT_N,attr"`
	PaymentSign       string `xml:"AT_SING,attr"`
	Number            string `xml:"N,attr"`
	Name              string `xml:"NM,attr"`
	AuthorizationCode string `xml:"PA,attr"`
	Card              string `xml:"PB,attr"`
	Terminal          string `xml:"PC,attr"`
	Transaction       string `xml:"PD,attr"`
	Extra             string `xml:"PE,attr"`
	PaymentSystem     string `xml:"PSNM,attr"`
	RRN               string `xml:"RRN,attr"`
	Total             string `xml:"SM,attr"`
	Type              string `xml:"T,attr"`
}

// ReceiptEndXML contains final receipt totals and summary information.
//
// This is where the receipt total, control number, local number, fiscal number,
// timestamp, and tax totals are usually stored.
type ReceiptEndXML struct {
	OfflineSessionNumber string          `xml:"AT_NOS,attr"`
	ControlNumber        string          `xml:"CS,attr"`
	FiscalNumber         string          `xml:"FN,attr"`
	Number               string          `xml:"N,attr"`
	LocalNumber          string          `xml:"NO,attr"`
	Total                string          `xml:"SM,attr"`
	Timestamp            string          `xml:"TS,attr"`
	Taxes                []ReceiptTaxXML `xml:"TX"`
}

// ReceiptTaxXML represents a tax summary row from the XML.
//
// Percent and total are strings at this layer because they require explicit
// conversion rules in the mapper.
type ReceiptTaxXML struct {
	Name    string `xml:"AT_NM,attr"`
	Label   string `xml:"AT_TL,attr"`
	Code    string `xml:"TX,attr"`
	Alias   string `xml:"TXAL,attr"`
	Percent string `xml:"TXPR,attr"`
	Total   string `xml:"TXSM,attr"`
	Type    string `xml:"TXTY,attr"`
}

// ReceiptXMLMAC represents the MAC/signature-like block from the XML.
//
// The text value is used as a control number in the normalized JSON response.
type ReceiptXMLMAC struct {
	DocumentID string `xml:"DI,attr"`
	Number     string `xml:"NT,attr"`
	Text       string `xml:",chardata"`
}

// ReceiptJSON is the normalized receipt model returned by our API.
//
// This model should not expose short XML/API-specific names. Any transformation
// from external API fields to clean application-level names should happen before
// data reaches this structure.
type ReceiptJSON struct {
	FiscalNumber      string          `json:"fiscalNumber"`
	LocalNumber       string          `json:"localNumber"`
	Total             float64         `json:"total"`
	DateTime          string          `json:"dateTime"`
	TaxNumber         string          `json:"taxNumber"`
	FactoryNumber     string          `json:"factoryNumber"`
	ReceiptText       string          `json:"receiptText"`
	IsFiscal          bool            `json:"isFiscal"`
	Products          []ProductJSON   `json:"products"`
	Discounts         []DiscountJSON  `json:"discounts"`
	Payments          []PaymentJSON   `json:"payments"`
	Taxes             []TaxJSON       `json:"taxes"`
	ControlNumber     string          `json:"controlNumber"`
	RawXML            string          `json:"rawXml"`
	OriginalAPIResult ReceiptResponse `json:"originalApiResult"`
}

// ProductJSON is the normalized product item returned to API clients.
//
// Quantity and Price are pointers because these fields may be absent in the XML.
// A missing value is different from a real zero value, so omitempty works
// correctly only when the field can be nil.
type ProductJSON struct {
	Number      string   `json:"number"`
	Code        string   `json:"code,omitempty"`
	Barcode     string   `json:"barcode,omitempty"`
	Name        string   `json:"name"`
	Quantity    *float64 `json:"quantity,omitempty"`
	Price       *float64 `json:"price,omitempty"`
	Total       float64  `json:"total"`
	TaxGroup    string   `json:"taxGroup,omitempty"`
	MeasureUnit string   `json:"measureUnit,omitempty"`
}

// DiscountJSON is the normalized discount model.
//
// Items contains product line numbers affected by the discount.
type DiscountJSON struct {
	Number string   `json:"number"`
	Total  float64  `json:"total"`
	Type   string   `json:"type,omitempty"`
	Tax    string   `json:"tax,omitempty"`
	Items  []string `json:"items,omitempty"`
}

// PaymentJSON is the normalized payment model.
//
// Optional fields are omitted from JSON when the external API does not provide
// terminal/card/payment-system details.
type PaymentJSON struct {
	Number            string  `json:"number"`
	Name              string  `json:"name,omitempty"`
	PaymentSystem     string  `json:"paymentSystem,omitempty"`
	Terminal          string  `json:"terminal,omitempty"`
	Card              string  `json:"card,omitempty"`
	AuthorizationCode string  `json:"authorizationCode,omitempty"`
	Transaction       string  `json:"transaction,omitempty"`
	Total             float64 `json:"total"`
}

// TaxJSON is the normalized tax summary returned to API clients.
type TaxJSON struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
	Total   float64 `json:"total"`
}

// ReceiptResponse describes the raw response returned by the external tax API.
//
// This structure intentionally keeps API field names close to the external JSON
// contract. It is stored in ReceiptJSON as OriginalAPIResult for debugging,
// traceability, and transparency.
type ReceiptResponse struct {
	Check      string  `json:"check"`
	Fn         string  `json:"fn"`
	Name       *string `json:"name"`
	AddressGo  *string `json:"addressGo"`
	TypeGo     *string `json:"typeGo"`
	Tins       *string `json:"tins"`
	XML        bool    `json:"xml"`
	Sign       bool    `json:"sign"`
	QR         bool    `json:"qr"`
	ResultCode *string `json:"resultCode"`
	ResultText string  `json:"resultText"`
	CheckXML   string  `json:"checkXml"`
	CheckP7s   *string `json:"checkP7s"`
}

// ProcessReceiptResult is the per-receipt processing result.
//
// ReceiptJSON is a pointer because failed items should not return an empty
// receipt object. On success it contains normalized receipt data, on failure
// it is omitted from the JSON response.
type ProcessReceiptResult struct {
	Status      ReceiptStatus                                `json:"status"`
	Message     string                                       `json:"message"`
	ReceiptJSON *ReceiptJSON                                 `json:"receiptJson,omitempty"`
	Enrichment  *enrichment.BackendProductEnrichmentResponse `json:"enrichment,omitempty"`
}

// ProcessReceiptsRequest accepts QR URLs, not already parsed receipts.
//
// The name "receipts" can be kept for frontend compatibility, but semantically
// each item is a QR URL string that will be validated and processed by the server.
type ProcessReceiptsRequest struct {
	Receipts []string `json:"receipts"`
}

// ProcessReceiptsResponse is the batch response returned after processing QR URLs.
//
// Count should represent the number of processed input items, while Results keeps
// success or failure details for every receipt independently.
type ProcessReceiptsResponse struct {
	UserID  string                 `json:"userId"`
	Count   int                    `json:"count"`
	Results []ProcessReceiptResult `json:"results"`
}
