package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"profiles-api/db"
	"profiles-api/models"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	stateStore   = map[string]string{}
	stateStoreMu sync.Mutex
)

// GET
func GithubLogin(w http.ResponseWriter, r *http.Request) {
	// Generate the state and codeVerifier
	state := generateRandomString()
	codeVerifier := generateRandomString()

	// store it locally, use a mutex so prevent a lock when multiple users are using it
	stateStoreMu.Lock()
	stateStore[state] = codeVerifier
	defer stateStoreMu.Unlock()

	// Communicating with github
	challenge := generateCodeChallenge(codeVerifier)
	params := url.Values{}
	client_id := os.Getenv("CLIENT_ID")
	redirect_uri := os.Getenv("REDIRECT_URI")

	params.Set("client_id", client_id)
	params.Set("redirect_uri", redirect_uri)
	params.Set("scope", "user:email")
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	redirectURL := "https://github.com/login/oauth/authorize?" + params.Encode()

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// GET
func GithubCallback(w http.ResponseWriter, r *http.Request) {
	githubState := r.URL.Query().Get("state")
	githubCode := r.URL.Query().Get("code")

	stateStoreMu.Lock()
	codeVerifier, ok := stateStore[githubState]
	if ok {
		delete(stateStore, githubState)
	}
	stateStoreMu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "State not found",
		})
		return
	}

	accessToken, err := exchangeCodeForToken(githubCode, codeVerifier)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	githubUser, err := getGithubUser(accessToken)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	user, err := upsertUser(githubUser)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}
	if !user.IsActive {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: "User Not active ",
		})
		return
	}

	jwtaccessToken, err := generateAccessToken(user.ID.String(), user.Role)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	refreshToken, err := generateRefreshToken(user.ID.String())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	// write the respons back to our Client
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":        "success",
		"access_token":  jwtaccessToken,
		"refresh_token": refreshToken,
	})

}

// POST
func RefreshToken(w http.ResponseWriter, r *http.Request) {

}

// POST
func Logout(w http.ResponseWriter, r *http.Request) {

}

func generateRandomString() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func exchangeCodeForToken(code, verifier string) (string, error) {
	// 1. Define the endpoint
	tokenURL := "https://github.com/login/oauth/access_token"

	// 2. Prepare the form data (The Body)
	data := url.Values{}
	data.Set("client_id", os.Getenv("CLIENT_ID"))
	data.Set("code", code)
	data.Set("code_verifier", verifier)
	data.Set("redirect_uri", os.Getenv("REDIRECT_URI"))
	data.Set("client_secret", os.Getenv("CLIENT_SECRET"))
	// Note: GitHub also requires an "Accept" header to return JSON instead of string pairs

	// 3. Create the Request
	// We use strings.NewReader because Post expects an io.Reader
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	// 4. Set Headers
	// This tells GitHub what we are sending and what we want back
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "application/json")

	// 5. Execute the request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 6. Decode the response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.AccessToken, nil
}

func getGithubUser(accessToken string) (models.GithubUser, error) {
	var user models.GithubUser

	url := "https://api.github.com/user"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return models.GithubUser{}, err
	}

	// 2. Add your Headers
	// This is where you pass the Access Token you just got!
	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", "my-go-app") // GitHub requires a User-Agent header

	// 3. Execute the request using your client
	resp, err := client.Do(req)
	if err != nil {
		return models.GithubUser{}, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return models.GithubUser{}, err
	}

	return user, nil

}

func upsertUser(githubUser models.GithubUser) (models.User, error) {
	var user models.User
	newID := uuid.Must(uuid.NewV7()).String()
	githubID := fmt.Sprintf("%d", githubUser.ID)

	query := `
        INSERT INTO users (id, github_id, username, email, avatar_url, role, is_active, last_login_at, created_at)
        VALUES ($1, $2, $3, $4, $5, 'analyst', true, NOW(), NOW())
        ON CONFLICT (github_id) 
        DO UPDATE SET 
            last_login_at = NOW(),
            username = EXCLUDED.username, -- Optional: keep their name in sync
            avatar_url = EXCLUDED.avatar_url
        RETURNING id, role, is_active;
    `

	err := db.DB.QueryRow(
		query,
		newID,                // $1
		githubID,             // $2 (The GitHub string ID)
		githubUser.Login,     // $3
		githubUser.Email,     // $4
		githubUser.AvatarURL, // $5
	).Scan(&user.ID, &user.Role, &user.IsActive)

	if err != nil {
		return models.User{}, err
	}

	return user, nil
}

func generateAccessToken(userID, role string) (string, error) {
	// 1. Create the claims
	claims := models.MyClaim{
		ID:               userID,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(3 * time.Minute))},
	}

	// 2. Create the token using the HS256 algorithm
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. Sign the token with our secret key
	mySecret := []byte(os.Getenv("JWT_SECRET"))
	tokenString, err := token.SignedString(mySecret)

	return tokenString, err
}

func generateRefreshToken(userID string) (string, error) {
	token := generateRandomString()
	newID := uuid.Must(uuid.NewV7()).String()

	_, err := db.DB.Exec(`INSERT INTO tokens 
	(id, user_id, token, expires_at, created_at)
	VALUES ($1, $2, $3, NOW() + INTERVAL '5 minutes', NOW())`,
		newID, userID, token,
	)

	if err != nil {
		return "", err
	}

	return token, nil

}
