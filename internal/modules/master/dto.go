package master

import "github.com/google/uuid"

type ListQuery struct {
	Page  int
	Limit int
	Q     string
}

type ItemResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type ListResponse struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    []ItemResponse `json:"data"`
	Meta    PaginationMeta `json:"meta"`
}
