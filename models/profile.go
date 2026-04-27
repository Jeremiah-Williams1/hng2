package models

import (
	"time"
)

// Profile Struct
type Profile struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Gender             string    `json:"gender"`
	GenderProbability  float64   `json:"gender_probability"`
	Age                int       `json:"age"`
	AgeGroup           string    `json:"age_group"`
	CountryID          string    `json:"country_id"`
	CountryName        string    `json:"country_name"`
	CountryProbability float64   `json:"country_probability"`
	CreatedAt          time.Time `json:"created_at"`
}

// Post input
type Input struct {
	Name string `json:"name"`
}

// GenderAPI struct
type GenderAPIResp struct {
	Name        string  `json:"name"`
	Gender      *string `json:"gender"`
	Probability float64 `json:"probability"`
	Count       int     `json:"count"`
}

// AgifyAPI Response
type AgifyResp struct {
	Name string `json:"name"`
	Age  *int   `json:"age"`
}

// NationalizeAPI Response
type Country struct {
	CountryID   string  `json:"country_id"`
	Probability float64 `json:"probability"`
}

type NationalizeResp struct {
	Name    string    `json:"name"`
	Country []Country `json:"country"`
}

// Success Response
type SuccessResponse struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
}

type ExistResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type PaginationLinks struct {
	Self string  `json:"self"`
	Next *string `json:"next"`
	Prev *string `json:"prev"`
}

type GSuccessResponse struct {
	Status     string          `json:"status"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Link       PaginationLinks `json:"links"`
	Data       any             `json:"data"`
}

// Error REsponse
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Struct for the channel
type GenderResult struct {
	Data GenderAPIResp
	Err  error
}
type AgeResult struct {
	Data AgifyResp
	Err  error
}
type NationResult struct {
	Data NationalizeResp
	Err  error
}

type ProfileQueryParams struct {
	Gender                string
	CountryID             string
	AgeGroup              string
	MinAge                string
	MaxAge                string
	MinGenderProbability  string
	MinCountryProbability string
	SortBy                string
	OrderBy               string
}

type ProfileQueryResult struct {
	SQLQuery   string
	CountQuery string
	Args       []any
}
