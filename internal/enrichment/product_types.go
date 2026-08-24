package enrichment

// ProductType represents a specific product class detected from receipt text.
//
// ProductType should be more specific than Category, but it must not include
// brand, volume, weight, package size or flavor.
type ProductType string

const (
	ProductTypeSparklingWater ProductType = "sparkling_water"
	ProductTypeStillWater     ProductType = "still_water"
	ProductTypeJuice          ProductType = "juice"
	ProductTypeSoftDrink      ProductType = "soft_drink"
	ProductTypeMilk           ProductType = "milk"
	ProductTypeYogurt         ProductType = "yogurt"
	ProductTypeCheese         ProductType = "cheese"
	ProductTypeBread          ProductType = "bread"
	ProductTypeChocolate      ProductType = "chocolate"
	ProductTypeCandy          ProductType = "candy"
	ProductTypeUnknown        ProductType = "unknown"
)

// allProductTypes keeps prompt output deterministic.
//
// This list should be updated together with supportedProductTypes whenever the
// backend taxonomy is expanded.
var allProductTypes = []ProductType{
	ProductTypeSparklingWater,
	ProductTypeStillWater,
	ProductTypeJuice,
	ProductTypeSoftDrink,
	ProductTypeMilk,
	ProductTypeYogurt,
	ProductTypeCheese,
	ProductTypeBread,
	ProductTypeChocolate,
	ProductTypeCandy,
	ProductTypeUnknown,
}

var supportedProductTypes = map[ProductType]struct{}{
	ProductTypeSparklingWater: {},
	ProductTypeStillWater:     {},
	ProductTypeJuice:          {},
	ProductTypeSoftDrink:      {},
	ProductTypeMilk:           {},
	ProductTypeYogurt:         {},
	ProductTypeCheese:         {},
	ProductTypeBread:          {},
	ProductTypeChocolate:      {},
	ProductTypeCandy:          {},
	ProductTypeUnknown:        {},
}

func (t ProductType) String() string {
	return string(t)
}

// IsValid checks whether the product type is part of the backend taxonomy.
func (t ProductType) IsValid() bool {
	_, ok := supportedProductTypes[t]
	return ok
}

// SupportedProductTypes returns supported product types in stable order.
//
// Returning a copy protects internal package state from external mutation.
func SupportedProductTypes() []ProductType {
	productTypes := make([]ProductType, len(allProductTypes))
	copy(productTypes, allProductTypes)

	return productTypes
}
