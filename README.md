# Profiles API

This api is used to generate profiles of users given their name. It calls external APIs:
1. Genderizer : It returns the gender and a gender_probability
2. Agify: returns the age of that person
3. Nationalize: returns nationality of the person

the three APIs runs concurrently to give response with minimal latency.

The results are then saved to a postgres database.

## Running Locally

1. Clone the repo
2. Set up PostgreSQL and create the database the table needed is created within the code
3. Run the seed script: `go run cmd/seed/main.go`
4. Start the server: `go run main.go`

## Endpoints

### POST /api/profiles
- Creates the User profile and saves it to the database 
It saves only unique names 

- Request body example

- Response example

### GET /api/profiles
- Get all profiles of uses
- Query parameters: gender, age_group, country_id, min_age, max_age, min_gender_probability, min_country_probability, sort_by, order, page, limit
- Example: `GET /api/profiles?gender=male&country_id=NG&page=1&limit=10`

### GET /api/profiles/search
- Takes Natural Language and use it as a search key to query the db and return the rows that satsifies it 

- Parameter: q (natural language query)

- Example queries that work:
  - `?q=young males from nigeria`
  - `?q=females above 30`
  - `?q=adult males from kenya`

### GET /api/profiles/{id}
### DELETE /api/profiles/{id}
