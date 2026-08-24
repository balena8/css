package enrichment

// Category represents a broad product group used by the enrichment pipeline.
//
// Category is intentionally broader than ProductType. For example, both sparkling
// water and juice belong to drinks, but they are different product types.
type Category string

const (
	CategoryDrinks       Category = "drinks"
	CategoryDairy        Category = "dairy"
	CategoryBakery       Category = "bakery"
	CategorySweets       Category = "sweets"
	CategoryMeat         Category = "meat"
	CategoryFish         Category = "fish"
	CategoryGrocery      Category = "grocery"
	CategoryFruitsVeg    Category = "fruits_vegetables"
	CategoryHousehold    Category = "household"
	CategoryPersonalCare Category = "personal_care"
	CategoryMedicine     Category = "medicine"
	CategoryPackaging    Category = "packaging"
	CategoryService      Category = "service"
	CategoryOther        Category = "other"
	CategoryUnknown      Category = "unknown"
)

// allCategories defines deterministic category order for prompts and responses.
//
// Do not build prompt text by iterating over supportedCategories directly:
// Go map iteration order is randomized.
var allCategories = []Category{
	CategoryDrinks,
	CategoryDairy,
	CategoryBakery,
	CategorySweets,
	CategoryMeat,
	CategoryFish,
	CategoryGrocery,
	CategoryFruitsVeg,
	CategoryHousehold,
	CategoryPersonalCare,
	CategoryMedicine,
	CategoryPackaging,
	CategoryService,
	CategoryOther,
	CategoryUnknown,
}

var supportedCategories = map[Category]struct{}{
	CategoryDrinks:       {},
	CategoryDairy:        {},
	CategoryBakery:       {},
	CategorySweets:       {},
	CategoryMeat:         {},
	CategoryFish:         {},
	CategoryGrocery:      {},
	CategoryFruitsVeg:    {},
	CategoryHousehold:    {},
	CategoryPersonalCare: {},
	CategoryMedicine:     {},
	CategoryPackaging:    {},
	CategoryService:      {},
	CategoryOther:        {},
	CategoryUnknown:      {},
}

func (c Category) String() string {
	return string(c)
}

// IsValid checks whether the category is part of the backend taxonomy.
func (c Category) IsValid() bool {
	_, ok := supportedCategories[c]
	return ok
}

// SupportedCategories returns categories in a stable order.
//
// A copy is returned so callers cannot mutate the internal taxonomy slice.
func SupportedCategories() []Category {
	categories := make([]Category, len(allCategories))
	copy(categories, allCategories)

	return categories
}
