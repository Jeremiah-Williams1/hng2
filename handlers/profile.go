package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"profiles-api/db"
	"profiles-api/models"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Post handler
func CreateProfile(w http.ResponseWriter, r *http.Request) {

	var input models.Input
	err := json.NewDecoder(r.Body).Decode(&input)
	name := input.Name

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	if name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "No Name in the request",
		})
		return
	}

	// Logic to check if the name is already in the database
	var p models.Profile
	err = db.DB.QueryRow(`
        SELECT id, name, gender, gender_probability, sample_size, age, age_group, country_id, country_probability, created_at
        FROM profiles WHERE LOWER(name) = LOWER($1)`, name).
		Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.SampleSize,
			&p.Age, &p.AgeGroup, &p.CountryID, &p.CountryProbability, &p.CreatedAt)

	if err == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.ExistResponse{
			Status:  "success",
			Message: "Profile already exist",
			Data:    p,
		})
		return
	}

	if err != sql.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// call three APIs concurrently
	genderCh := make(chan models.GenderResult, 1)
	ageCh := make(chan models.AgeResult, 1)
	nationCh := make(chan models.NationResult, 1)

	go func() {
		data, err := CallGenderizer(name)
		genderCh <- models.GenderResult{
			Data: data,
			Err:  err,
		}
	}()

	go func() {
		data, err := CallAgify(name)
		ageCh <- models.AgeResult{
			Data: data,
			Err:  err,
		}
	}()

	go func() {
		data, err := CallNationalize(name)
		nationCh <- models.NationResult{
			Data: data,
			Err:  err,
		}
	}()

	gRes := <-genderCh
	aRes := <-ageCh
	nRes := <-nationCh

	// read from the channel and then run the check
	if gRes.Err != nil || gRes.Data.Gender == nil || gRes.Data.Count == 0 {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Genderize returned an invalid response",
		})
		return
	}

	if aRes.Err != nil || aRes.Data.Age == nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Agify returned an invalid response",
		})
		return
	}

	if nRes.Err != nil || len(nRes.Data.Country) == 0 {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Nationslize returned an invalid response",
		})
		return
	}

	// Conditions
	age := *aRes.Data.Age
	ageGroup := ageGroup(age)

	// select the nation with the highest probablity
	topCountry := nRes.Data.Country[0]
	for _, v := range nRes.Data.Country {
		if v.Probability > topCountry.Probability {
			topCountry = v
		}
	}

	// filling the response
	response := models.Profile{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		Name:               gRes.Data.Name,
		Gender:             *gRes.Data.Gender,
		GenderProbability:  gRes.Data.Probability,
		SampleSize:         gRes.Data.Count,
		Age:                *aRes.Data.Age,
		AgeGroup:           ageGroup,
		CountryID:          topCountry.CountryID,
		CountryProbability: topCountry.Probability,
		CreatedAt:          time.Now().UTC().Format(time.RFC3339),
	}

	// saving the db to the database
	_, err = db.DB.Exec(`
        INSERT INTO profiles 
        (id, name, gender, gender_probability, sample_size, age, age_group, country_id, country_probability, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		response.ID, response.Name, response.Gender, response.GenderProbability,
		response.SampleSize, response.Age, response.AgeGroup,
		response.CountryID, response.CountryProbability, response.CreatedAt,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Failed to save Profile",
		})
		return
	}

	// write the respons back to our Client
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.SuccessResponse{
		Status: "success",
		Data:   response,
	})

}

// Function that Request from the APIs
// Create the Client Object used for request
var client = &http.Client{Timeout: 5 * time.Second}

func CallGenderizer(name string) (models.GenderAPIResp, error) {

	// make the request
	url := fmt.Sprintf("https://api.genderize.io/?name=%s", name)
	resp, err := client.Get(url)
	if err != nil {
		return models.GenderAPIResp{}, err
	}

	// close the connection
	defer resp.Body.Close()

	// Additional check
	if resp.StatusCode != http.StatusOK {
		return models.GenderAPIResp{}, err
	}

	var result models.GenderAPIResp
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return models.GenderAPIResp{}, err
	}

	return result, nil

}

func CallAgify(name string) (models.AgifyResp, error) {
	// make the request
	url := fmt.Sprintf("https://api.agify.io/?name=%s", name)
	resp, err := client.Get(url)
	if err != nil {
		return models.AgifyResp{}, err
	}

	// close the connection
	defer resp.Body.Close()

	// Additional check
	if resp.StatusCode != http.StatusOK {
		return models.AgifyResp{}, err
	}

	var result models.AgifyResp
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return models.AgifyResp{}, err
	}

	return result, nil
}

func CallNationalize(name string) (models.NationalizeResp, error) {
	url := fmt.Sprintf("https://api.nationalize.io/?name=%s", name)
	resp, err := client.Get(url)
	if err != nil {
		return models.NationalizeResp{}, err
	}
	defer resp.Body.Close()
	var result models.NationalizeResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return models.NationalizeResp{}, err
	}
	return result, nil
}

func ageGroup(age int) string {
	switch {
	case age <= 12:
		return "child"
	case age <= 19:
		return "teenager"
	case age <= 59:
		return "adult"
	default:
		return "senior"
	}
}

// Get Single Profile
func GetProfileById(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// use sql to get it from the database
	row := db.DB.QueryRow(`
        SELECT id, name, gender, gender_probability, sample_size, age, age_group, country_id, country_probability, created_at
        FROM profiles WHERE id = $1`, id)

	var resp models.Profile
	err := row.Scan(&resp.ID, &resp.Name, &resp.Gender, &resp.GenderProbability, &resp.SampleSize,
		&resp.Age, &resp.AgeGroup, &resp.CountryID, &resp.CountryProbability, &resp.CreatedAt)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Note with that ID isn't available",
		})
		return
	}

	// return it.
	w.WriteHeader(http.StatusOK)

	// encode and send response back
	json.NewEncoder(w).Encode(models.SuccessResponse{
		Status: "success",
		Data:   resp,
	})
}

// Get for multiple profiles or all
func GetProfiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gender := strings.ToLower(query.Get("gender"))
	countryID := strings.ToUpper(query.Get("country_id"))
	ageGroup := strings.ToLower(query.Get("age_group"))

	// Build query dynamically
	sqlQuery := `SELECT id, name, gender, age, age_group, country_id FROM profiles WHERE 1=1`
	args := []any{}
	i := 1

	if gender != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(gender) = $%d", i)
		args = append(args, gender)
		i++
	}
	if countryID != "" {
		sqlQuery += fmt.Sprintf(" AND UPPER(country_id) = $%d", i)
		args = append(args, countryID)
		i++
	}
	if ageGroup != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(age_group) = $%d", i)
		args = append(args, ageGroup)
		i++
	}

	rows, err := db.DB.Query(sqlQuery, args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	// Separate list struct — only the fields the spec wants
	type listProfile struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Gender    string `json:"gender"`
		Age       int    `json:"age"`
		AgeGroup  string `json:"age_group"`
		CountryID string `json:"country_id"`
	}

	profiles := make([]listProfile, 0)
	for rows.Next() {
		var p listProfile
		err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.Age, &p.AgeGroup, &p.CountryID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		profiles = append(profiles, p)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.GSuccessResponse{
		Status: "success",
		Count:  len(profiles),
		Data:   profiles,
	})
}

// Delete a profile
func DeleteProfile(w http.ResponseWriter, r *http.Request) {
	// get the id
	id := r.PathValue("id")

	// get that profile from the db
	result, err := db.DB.Exec("DELETE FROM profiles WHERE id = $1", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	count, err := result.RowsAffected()
	if count == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Profile not found",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)

}
