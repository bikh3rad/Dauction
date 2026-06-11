package dto

// CategoryResp is the language-neutral category vocabulary entry. `code` is the
// stable machine key; `iconKey` maps to the design-team per-category icon set.
// Display names are localized client-side (CLAUDE.md §7).
type CategoryResp struct {
	Code      string `json:"code"`
	IconKey   string `json:"iconKey"`
	SortOrder int    `json:"sortOrder"`
}

// SeededCategories returns the catalog's category vocabulary, matching the seed
// in migration 000005_categories_inspection.up.sql. Categories are effectively
// static config; serving them here avoids a per-request DB read.
func SeededCategories() []CategoryResp {
	return []CategoryResp{
		{Code: "WATCHES", IconKey: "cat-watches", SortOrder: 10},
		{Code: "JEWELRY", IconKey: "cat-jewelry", SortOrder: 20},
		{Code: "FINE_ART", IconKey: "cat-fine-art", SortOrder: 30},
		{Code: "AUTOMOBILES", IconKey: "cat-automobiles", SortOrder: 40},
		{Code: "HANDBAGS", IconKey: "cat-handbags", SortOrder: 50},
		{Code: "RARE_SPIRITS", IconKey: "cat-rare-spirits", SortOrder: 60},
		{Code: "COLLECTIBLES", IconKey: "cat-collectibles", SortOrder: 70},
	}
}
