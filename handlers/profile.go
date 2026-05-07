package handlers

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"profiles-api/cache"
	"profiles-api/db"
	"profiles-api/models"
	"profiles-api/queries"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var countryNames = map[string]string{
	"NG": "Nigeria",
	"US": "United States",
	"GB": "United Kingdom",
	"IN": "India",
	"BR": "Brazil",
	"DE": "Germany",
	"FR": "France",
	"JP": "Japan",
	"CN": "China",
	"MX": "Mexico",
	"KE": "Kenya",
	"GH": "Ghana",
	"ZA": "South Africa",
	"EG": "Egypt",
	"AO": "Angola",
	"CA": "Canada",
	"AU": "Australia",
	"ID": "Indonesia",
	"PK": "Pakistan",
	"RU": "Russia",
	"ES": "Spain",
	"IT": "Italy",
	"KR": "South Korea",
	"TR": "Turkey",
	"SA": "Saudi Arabia",
	"AR": "Argentina",
	"NL": "Netherlands",
	"AE": "United Arab Emirates",
	"SG": "Singapore",
	"ET": "Ethiopia",
}

// Helper function
func getCountryName(code string) string {
	if name, ok := countryNames[code]; ok {
		return name
	}
	return code // fallback: return the code itself
}

// getCountryID reverses the countryNames map: "nigeria" -> "NG"
func getCountryID(name string) (string, bool) {
	lower := strings.ToLower(name)
	for code, countryName := range countryNames {
		if strings.ToLower(countryName) == lower {
			return code, true
		}
	}
	return "", false
}

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
        SELECT id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at
        FROM profiles WHERE LOWER(name) = LOWER($1)`, name).
		Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability,
			&p.Age, &p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt)

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

	countryName := getCountryName(topCountry.CountryID)

	// filling the response
	response := models.Profile{
		ID:                 uuid.Must(uuid.NewV7()).String(),
		Name:               gRes.Data.Name,
		Gender:             *gRes.Data.Gender,
		GenderProbability:  gRes.Data.Probability,
		Age:                *aRes.Data.Age,
		AgeGroup:           ageGroup,
		CountryID:          topCountry.CountryID,
		CountryName:        countryName,
		CountryProbability: topCountry.Probability,
		CreatedAt:          time.Now().UTC(),
	}

	// saving the db to the database
	_, err = db.DB.Exec(`
        INSERT INTO profiles 
        (id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		response.ID, response.Name, response.Gender, response.GenderProbability,
		response.Age, response.AgeGroup,
		response.CountryID, response.CountryName, response.CountryProbability, response.CreatedAt,
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
        SELECT id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at
        FROM profiles WHERE id = $1`, id)

	var resp models.Profile
	err := row.Scan(&resp.ID, &resp.Name, &resp.Gender, &resp.GenderProbability,
		&resp.Age, &resp.AgeGroup, &resp.CountryID, &resp.CountryName, &resp.CountryProbability, &resp.CreatedAt)
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "Data not in Database",
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

	params := map[string]string{
		"gender":                  strings.ToLower(query.Get("gender")),
		"country_id":              strings.ToUpper(query.Get("country_id")),
		"age_group":               strings.ToLower(query.Get("age_group")),
		"min_age":                 query.Get("min_age"),
		"max_age":                 query.Get("max_age"),
		"min_gender_probability":  query.Get("min_gender_probability"),
		"min_country_probability": query.Get("min_country_probability"),
		"sort_by":                 strings.ToLower(query.Get("sort_by")),
		"order":                   strings.ToLower(query.Get("order")),
		"page":                    query.Get("page"),
		"limit":                   query.Get("limit"),
	}

	cacheKey := "get_profiles:" + cache.BuildCacheKey(params)

	if cached, ok := cache.ProfileCache.Get(cacheKey); ok {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cached)
		return
	}

	builtQuery, err := BuildProfileQuery(models.ProfileQueryParams{
		Gender:                params["gender"],
		CountryID:             params["country_id"],
		AgeGroup:              params["age_group"],
		MinAge:                params["min_age"],
		MaxAge:                params["max_age"],
		MinGenderProbability:  params["min_gender_probability"],
		MinCountryProbability: params["min_country_probability"],
		SortBy:                params["sort_by"],
		OrderBy:               params["order"],
	})
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: "Invalid query parameters",
		})
		return
	}

	var page, limit int = 1, 10
	numPage := query.Get("page")
	numLimit := query.Get("limit")

	var totalCount int
	err = db.DB.QueryRow(builtQuery.CountQuery, builtQuery.Args...).Scan(&totalCount)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: err.Error(),
		})
		return
	}

	if numPage != "" {
		page, err = strconv.Atoi(numPage)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: "Invalid query parameters",
			})
			return
		}
	}
	if numLimit != "" {
		limit, err = strconv.Atoi(numLimit)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: "Invalid query parameters",
			})
			return
		}
		if limit > 50 {
			limit = 50
		}
	}

	offset := (page - 1) * limit
	i := len(builtQuery.Args) + 1
	sqlQuery := builtQuery.SQLQuery
	args := builtQuery.Args

	sqlQuery += fmt.Sprintf(" LIMIT $%d", i)
	args = append(args, limit)
	i++

	sqlQuery += fmt.Sprintf(" OFFSET $%d", i)
	args = append(args, offset)

	rows, err := db.DB.Query(sqlQuery, args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	profiles := make([]models.Profile, 0)
	for rows.Next() {
		var p models.Profile
		err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.Age, &p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: err.Error(),
			})
			return
		}
		profiles = append(profiles, p)
	}
	totalPages := (totalCount + limit - 1) / limit
	buildURL := func(p int) string {
		return fmt.Sprintf("%s?page=%d&limit=%d", r.URL.Path, p, limit)
	}

	links := models.PaginationLinks{
		Self: buildURL(page),
	}
	// Handle Next link
	if page < totalPages {
		nextAddr := buildURL(page + 1)
		links.Next = &nextAddr
	}

	// Handle Prev link
	if page > 1 {
		prevAddr := buildURL(page - 1)
		links.Prev = &prevAddr
	}

	response := models.GSuccessResponse{
		Status:     "success",
		Page:       page,
		Limit:      limit,
		Total:      totalCount,
		TotalPages: totalPages,
		Link:       links,
		Data:       profiles,
	}

	cache.ProfileCache.Set(cacheKey, response)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
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

	count, _ := result.RowsAffected()
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

func SearchProfiles(w http.ResponseWriter, r *http.Request) {
	rawQ := strings.TrimSpace(r.URL.Query().Get("q"))
	if rawQ == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: "Unable to interpret query",
		})
		return
	}

	parsed, ok := queries.ParseNLQuery(rawQ, getCountryID)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: "Unable to interpret query",
		})
		return
	}

	// Caching implementation
	params := map[string]string{
		"gender":     parsed.Gender,
		"country_id": parsed.CountryID,
		"age_group":  parsed.AgeGroup,
		"min_age":    parsed.MinAge,
		"max_age":    parsed.MaxAge,
		"page":       r.URL.Query().Get("page"),
		"limit":      r.URL.Query().Get("limit"),
	}

	cacheKey := "search_profiles:" + cache.BuildCacheKey(params)

	if cached, ok := cache.ProfileCache.Get(cacheKey); ok {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cached)
		return
	}

	builtQuery, err := BuildProfileQuery(models.ProfileQueryParams{
		Gender:    parsed.Gender,
		CountryID: parsed.CountryID,
		AgeGroup:  parsed.AgeGroup,
		MinAge:    parsed.MinAge,
		MaxAge:    parsed.MaxAge,
	})
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: "Invalid query parameters",
		})
		return
	}

	query := r.URL.Query()
	var page, limit int = 1, 10

	if numPage := query.Get("page"); numPage != "" {
		page, err = strconv.Atoi(numPage)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: "Invalid query parameters",
			})
			return
		}
	}
	if numLimit := query.Get("limit"); numLimit != "" {
		limit, err = strconv.Atoi(numLimit)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: "Invalid query parameters",
			})
			return
		}
		if limit > 50 {
			limit = 50
		}
	}

	var totalCount int
	err = db.DB.QueryRow(builtQuery.CountQuery, builtQuery.Args...).Scan(&totalCount)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: err.Error(),
		})
		return
	}

	offset := (page - 1) * limit
	i := len(builtQuery.Args) + 1
	sqlQuery := builtQuery.SQLQuery
	args := builtQuery.Args

	sqlQuery += fmt.Sprintf(" LIMIT $%d", i)
	args = append(args, limit)
	i++
	sqlQuery += fmt.Sprintf(" OFFSET $%d", i)
	args = append(args, offset)

	rows, err := db.DB.Query(sqlQuery, args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status: "error", Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	profiles := make([]models.Profile, 0)
	for rows.Next() {
		var p models.Profile
		err := rows.Scan(&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.Age, &p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status: "error", Message: err.Error(),
			})
			return
		}
		profiles = append(profiles, p)
	}

	totalPages := (totalCount + limit - 1) / limit
	buildURL := func(p int) string {
		return fmt.Sprintf("%s?page=%d&limit=%d", r.URL.Path, p, limit)
	}

	links := models.PaginationLinks{
		Self: buildURL(page),
	}
	// Handle Next link
	if page < totalPages {
		nextAddr := buildURL(page + 1)
		links.Next = &nextAddr
	}

	// Handle Prev link
	if page > 1 {
		prevAddr := buildURL(page - 1)
		links.Prev = &prevAddr
	}

	response := models.GSuccessResponse{
		Status:     "success",
		Page:       page,
		Limit:      limit,
		Total:      totalCount,
		TotalPages: totalPages,
		Link:       links,
		Data:       profiles,
	}

	cache.ProfileCache.Set(cacheKey, response)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Export csv Endpoint
func ExportProfiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	format := query.Get("format")
	if format != "csv" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "Unsupported format, use format=csv"})
		return
	}

	// 1. Reuse your existing filter/sort logic
	builtQuery, err := BuildProfileQuery(models.ProfileQueryParams{
		Gender:                strings.ToLower(query.Get("gender")),
		CountryID:             strings.ToUpper(query.Get("country_id")),
		AgeGroup:              strings.ToLower(query.Get("age_group")),
		MinAge:                query.Get("min_age"),
		MaxAge:                query.Get("max_age"),
		MinGenderProbability:  query.Get("min_gender_probability"),
		MinCountryProbability: query.Get("min_country_probability"),
		SortBy:                strings.ToLower(query.Get("sort_by")),
		OrderBy:               strings.ToLower(query.Get("order")),
	})
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "Invalid query parameters"})
		return
	}

	// 2. Execute query (No LIMIT/OFFSET for export)
	rows, err := db.DB.Query(builtQuery.SQLQuery, builtQuery.Args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	defer rows.Close()

	// 3. Set CSV Headers
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="profiles_%s.csv"`, time.Now().Format("20060102_150405")))

	// 4. Initialize CSV Writer
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write Header Row
	header := []string{
		"id", "name", "gender", "gender_probability", "age",
		"age_group", "country_id", "country_name", "country_probability", "created_at",
	}
	err = writer.Write(header)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// 5. Stream Rows to CSV
	for rows.Next() {
		var p models.Profile
		err := rows.Scan(
			&p.ID, &p.Name, &p.Gender, &p.GenderProbability, &p.Age,
			&p.AgeGroup, &p.CountryID, &p.CountryName, &p.CountryProbability, &p.CreatedAt,
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		record := []string{
			p.ID,
			p.Name,
			p.Gender,
			fmt.Sprintf("%.2f", p.GenderProbability),
			strconv.Itoa(p.Age),
			p.AgeGroup,
			p.CountryID,
			p.CountryName,
			fmt.Sprintf("%.2f", p.CountryProbability),
			p.CreatedAt.Format(time.RFC3339),
		}
		writer.Write(record)
	}
}

// Helper function
func BuildProfileQuery(p models.ProfileQueryParams) (models.ProfileQueryResult, error) {
	sqlQuery := `SELECT id, name, gender, gender_probability, age, age_group, country_id, 
    country_name, country_probability, created_at
    FROM profiles WHERE 1=1`
	countQuery := "SELECT COUNT(*) FROM profiles WHERE 1=1"

	args := []any{}
	i := 1

	if p.Gender != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(gender) = $%d", i)
		countQuery += fmt.Sprintf(" AND LOWER(gender) = $%d", i)
		args = append(args, p.Gender)
		i++
	}
	if p.CountryID != "" {
		sqlQuery += fmt.Sprintf(" AND country_id = $%d", i)
		countQuery += fmt.Sprintf(" AND country_id = $%d", i)
		args = append(args, p.CountryID)
		i++
	}
	if p.AgeGroup != "" {
		sqlQuery += fmt.Sprintf(" AND LOWER(age_group) = $%d", i)
		countQuery += fmt.Sprintf(" AND LOWER(age_group) = $%d", i)
		args = append(args, p.AgeGroup)
		i++
	}
	if p.MinAge != "" {
		val, err := strconv.Atoi(p.MinAge)
		if err != nil {
			return models.ProfileQueryResult{}, fmt.Errorf("invalid min_age")
		}
		sqlQuery += fmt.Sprintf(" AND age >= $%d", i)
		countQuery += fmt.Sprintf(" AND age >= $%d", i)
		args = append(args, val)
		i++
	}
	if p.MaxAge != "" {
		val, err := strconv.Atoi(p.MaxAge)
		if err != nil {
			return models.ProfileQueryResult{}, fmt.Errorf("invalid max_age")
		}
		sqlQuery += fmt.Sprintf(" AND age <= $%d", i)
		countQuery += fmt.Sprintf(" AND age <= $%d", i)
		args = append(args, val)
		i++
	}
	if p.MinCountryProbability != "" {
		val, err := strconv.ParseFloat(p.MinCountryProbability, 64)
		if err != nil {
			return models.ProfileQueryResult{}, fmt.Errorf("invalid min_country_probability")
		}
		sqlQuery += fmt.Sprintf(" AND country_probability >= $%d", i)
		countQuery += fmt.Sprintf(" AND country_probability >= $%d", i)
		args = append(args, val)
		i++
	}
	if p.MinGenderProbability != "" {
		val, err := strconv.ParseFloat(p.MinGenderProbability, 64)
		if err != nil {
			return models.ProfileQueryResult{}, fmt.Errorf("invalid min_gender_probability")
		}
		sqlQuery += fmt.Sprintf(" AND gender_probability >= $%d", i)
		countQuery += fmt.Sprintf(" AND gender_probability >= $%d", i)
		args = append(args, val)
		i++
	}
	if p.SortBy != "" {
		column := "created_at"
		switch p.SortBy {
		case "age":
			column = "age"
		case "created_at":
			column = "created_at"
		case "gender_probability":
			column = "gender_probability"
		}
		sortOrder := "ASC"
		if p.OrderBy == "desc" {
			sortOrder = "DESC"
		}
		sqlQuery += fmt.Sprintf(" ORDER BY %s %s", column, sortOrder)
	}

	return models.ProfileQueryResult{
		SQLQuery:   sqlQuery,
		CountQuery: countQuery,
		Args:       args,
	}, nil
}

func UploadProfiles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20) // 512MB cap

	err := r.ParseMultipartForm(32 << 20) // 32MB in memory, rest to disk
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "File too large or malformed"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "Missing file field"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // don't panic on inconsistent column counts, we'll validate manually

	// read and validate header row
	header, err := reader.Read()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "Cannot read CSV header"})
		return
	}

	expectedCols := []string{"name", "gender", "age", "country_id"}
	colIndex := map[string]int{}
	for i, col := range header {
		colIndex[strings.ToLower(strings.TrimSpace(col))] = i
	}
	for _, required := range expectedCols {
		if _, ok := colIndex[required]; !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Status: "error", Message: "Missing required column: " + required})
			return
		}
	}

	type rowResult struct {
		inserted int
		skipped  int
		reasons  map[string]int
	}

	// process in chunks
	const chunkSize = 500
	totalRows := 0
	totalInserted := 0
	totalSkipped := 0
	reasons := map[string]int{}

	type profileRow struct {
		name      string
		gender    string
		age       int
		countryID string
	}

	chunk := make([]profileRow, 0, chunkSize)

	flushChunk := func(rows []profileRow) (int, int, map[string]int) {
		if len(rows) == 0 {
			return 0, 0, map[string]int{}
		}

		// build bulk INSERT with ON CONFLICT DO NOTHING
		valueStrings := make([]string, 0, len(rows))
		valueArgs := make([]any, 0, len(rows)*4)
		idx := 1
		for _, row := range rows {
			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d, $%d, $%d, $%d)", idx, idx+1, idx+2, idx+3))
			valueArgs = append(valueArgs, row.name, row.gender, row.age, row.countryID)
			idx += 4
		}

		sqlStr := "INSERT INTO profiles (name, gender, age, country_id) VALUES " +
			strings.Join(valueStrings, ",") +
			" ON CONFLICT (name) DO NOTHING"

		result, err := db.DB.Exec(sqlStr, valueArgs...)
		if err != nil {
			// entire chunk failed — count all as skipped
			return 0, len(rows), map[string]int{"db_error": len(rows)}
		}

		affected, _ := result.RowsAffected()
		duplicates := len(rows) - int(affected)
		skippedReasons := map[string]int{}
		if duplicates > 0 {
			skippedReasons["duplicate_name"] = duplicates
		}
		return int(affected), duplicates, skippedReasons
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			totalRows++
			totalSkipped++
			reasons["malformed_row"]++
			continue
		}

		totalRows++

		// validate column count
		if len(record) < len(expectedCols) {
			totalSkipped++
			reasons["missing_fields"]++
			continue
		}

		name := strings.TrimSpace(record[colIndex["name"]])
		gender := strings.ToLower(strings.TrimSpace(record[colIndex["gender"]]))
		ageStr := strings.TrimSpace(record[colIndex["age"]])
		countryID := strings.ToUpper(strings.TrimSpace(record[colIndex["country_id"]]))

		// validate fields
		if name == "" || gender == "" || ageStr == "" || countryID == "" {
			totalSkipped++
			reasons["missing_fields"]++
			continue
		}

		age, err := strconv.Atoi(ageStr)
		if err != nil || age < 0 || age > 150 {
			totalSkipped++
			reasons["invalid_age"]++
			continue
		}

		if gender != "male" && gender != "female" {
			totalSkipped++
			reasons["invalid_gender"]++
			continue
		}

		chunk = append(chunk, profileRow{name, gender, age, countryID})

		if len(chunk) == chunkSize {
			ins, skp, rsns := flushChunk(chunk)
			totalInserted += ins
			totalSkipped += skp
			for k, v := range rsns {
				reasons[k] += v
			}
			chunk = chunk[:0] // reset slice, reuse backing array
		}
	}

	// flush remaining rows
	ins, skp, rsns := flushChunk(chunk)
	totalInserted += ins
	totalSkipped += skp
	for k, v := range rsns {
		reasons[k] += v
	}

	// invalidate cache so next read sees fresh data
	cache.ProfileCache.Invalidate()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":     "success",
		"total_rows": totalRows,
		"inserted":   totalInserted,
		"skipped":    totalSkipped,
		"reasons":    reasons,
	})
}
