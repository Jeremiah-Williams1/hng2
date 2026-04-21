package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"profiles-api/db"
	"profiles-api/models"

	"github.com/google/uuid"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://jeremiah:mypassword123@localhost:5432/profiles_db"
	}

	err := db.Connect(connStr)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	type person struct {
		Name               string  `json:"name"`
		Gender             string  `json:"gender"`
		GenderProbability  float64 `json:"gender_probability"`
		Age                int     `json:"age"`
		AgeGroup           string  `json:"age_group"`
		CountryID          string  `json:"country_id"`
		CountryName        string  `json:"country_name"`
		CountryProbability float64 `json:"country_probability"`
	}

	// var client = &http.Client{Timeout: 5 * time.Second}

	// // make the request
	// url := "https://drive.google.com/file/d/1Up06dcS9OfUEnDj_u6OV_xTRntupFhPH/view"
	// resp, err := client.Get(url)
	// if err != nil {
	// 	log.Fatal(err.Error())
	// 	return
	// }

	// // close the connection
	// defer resp.Body.Close()

	// // Additional check
	// if resp.StatusCode != http.StatusOK {
	// 	log.Fatal("Request failed")
	// 	return
	// }

	var result struct {
		Profiles []person `json:"profiles"`
	}

	file, err := os.Open("profiles.json")
	if err != nil {
		log.Fatal("Could not open file:", err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		log.Fatal(err.Error())
		return
	}

	for _, v := range result.Profiles {
		profile := models.Profile{
			ID:                 uuid.Must(uuid.NewV7()).String(),
			Name:               v.Name,
			Gender:             v.Gender,
			GenderProbability:  v.GenderProbability,
			Age:                v.Age,
			AgeGroup:           v.AgeGroup,
			CountryID:          v.CountryID,
			CountryName:        v.CountryName,
			CountryProbability: v.CountryProbability,
			CreatedAt:          time.Now().UTC(),
		}

		// saving the db to the database
		_, err = db.DB.Exec(`
        INSERT INTO profiles 
        (id, name, gender, gender_probability, age, age_group, country_id, country_name, country_probability, created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT DO NOTHING`,
			profile.ID, profile.Name, profile.Gender, profile.GenderProbability,
			profile.Age, profile.AgeGroup,
			profile.CountryID, profile.CountryName, profile.CountryProbability, profile.CreatedAt,
		)

		if err != nil {
			log.Fatal(err.Error())
			return
		}
	}

}
