package master

func mapCatalogItems(entities []CatalogItem) []ItemResponse {
	items := make([]ItemResponse, 0, len(entities))
	for _, entity := range entities {
		items = append(items, ItemResponse{ID: entity.ID, Name: entity.Name, Slug: entity.Slug})
	}
	return items
}
