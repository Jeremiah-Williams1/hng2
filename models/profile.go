package models

// Profile Struct
type Profile struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Gender             string  `json:"gender"`
	GenderProbability  float64 `json:"gender_probability"`
	SampleSize         int     `json:"sample_size"`
	Age                int     `json:"age"`
	AgeGroup           string  `json:"age_group"`
	CountryID          string  `json:"country_id"`
	CountryProbability float64 `json:"country_probability"`
	CreatedAt          string  `json:"created_at"`
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

type GSuccessResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
	Data   any    `json:"data"`
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
