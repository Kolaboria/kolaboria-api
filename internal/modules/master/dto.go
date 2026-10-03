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

type SearchLocationResponse struct {
	VillageID  int64  `json:"village_id" gorm:"column:village_id"`
	Village    string `json:"village" gorm:"column:village"`
	District   string `json:"district" gorm:"column:district"`
	Regency    string `json:"regency" gorm:"column:regency"`
	Province   string `json:"province" gorm:"column:province"`
	PostalCode string `json:"postal_code" gorm:"column:postal_code"`
}

type LocationInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type LocationDetailResponse struct {
	Province   LocationInfo `json:"province"`
	Regency    LocationInfo `json:"regency"`
	District   LocationInfo `json:"district"`
	Village    LocationInfo `json:"village"`
	PostalCode string       `json:"postal_code"`
}

type SearchLocationListResponse struct {
	Status  int                      `json:"status"`
	Message string                   `json:"message"`
	Data    []SearchLocationResponse `json:"data"`
}

type LocationDetailResultResponse struct {
	Status  int                    `json:"status"`
	Message string                 `json:"message"`
	Data    LocationDetailResponse `json:"data"`
}

