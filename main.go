package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// User представляет пользователя
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"-"`
}

// Ad представляет объявление
type Ad struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	Price       float64   `json:"price"`
	AuthorID    int       `json:"author_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// Token представляет авторизационный токен
type Token struct {
	Value string `json:"token"`
}

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("postgres", "postgres://mihailmamaev:@localhost/marketplace?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/register", RegisterHandler).Methods("POST")
	router.HandleFunc("/login", LoginHandler).Methods("POST")

	authRouter := router.PathPrefix("/api").Subrouter()
	authRouter.Use(AuthMiddleware)

	authRouter.HandleFunc("/ad", CreateAdHandler).Methods("POST")
	authRouter.HandleFunc("/ads", GetAdsHandler).Methods("GET")

	fmt.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// RegisterHandler обрабатывает регистрацию пользователей
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	query := "INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id"
	err = db.QueryRow(query, user.Username, user.Password).Scan(&user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(user)
}

// LoginHandler обрабатывает авторизацию пользователей
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var storedUser User
	query := "SELECT id, username FROM users WHERE username=$1 AND password=$2"
	err = db.QueryRow(query, user.Username, user.Password).Scan(&storedUser.ID, &storedUser.Username)
	if err != nil {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	token := generateToken()
	_, err = db.Exec("INSERT INTO tokens (value, user_id) VALUES ($1, $2)", token, storedUser.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := Token{Value: token}
	json.NewEncoder(w).Encode(response)
}

// CreateAdHandler обрабатывает создание объявлений
func CreateAdHandler(w http.ResponseWriter, r *http.Request) {
	var ad Ad
	err := json.NewDecoder(r.Body).Decode(&ad)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	username := getUsernameFromToken(r)
	var authorID int
	err = db.QueryRow("SELECT id FROM users WHERE username=$1", username).Scan(&authorID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	query := "INSERT INTO ads (title, description, image_url, price, author_id, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id"
	err = db.QueryRow(query, ad.Title, ad.Description, ad.ImageURL, ad.Price, authorID, time.Now()).Scan(&ad.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ad.AuthorID = authorID
	ad.CreatedAt = time.Now()
	json.NewEncoder(w).Encode(ad)
}

// GetAdsHandler обрабатывает получение списка объявлений
func GetAdsHandler(w http.ResponseWriter, r *http.Request) {
	// Implement pagination, sorting, and filtering
	rows, err := db.Query("SELECT id, title, description, image_url, price, author_id, created_at FROM ads")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var ads []Ad
	for rows.Next() {
		var ad Ad
		err := rows.Scan(&ad.ID, &ad.Title, &ad.Description, &ad.ImageURL, &ad.Price, &ad.AuthorID, &ad.CreatedAt)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ads = append(ads, ad)
	}

	json.NewEncoder(w).Encode(ads)
}

// AuthMiddleware проверяет авторизацию пользователя
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		var userID int
		err := db.QueryRow("SELECT user_id FROM tokens WHERE value=$1", token).Scan(&userID)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func generateToken() string {
	return "randomtoken" // Placeholder for actual token generation logic
}

func getUsernameFromToken(r *http.Request) string {
	token := r.Header.Get("Authorization")
	var username string
	db.QueryRow("SELECT username FROM users WHERE id=(SELECT user_id FROM tokens WHERE value=$1)", token).Scan(&username)
	return username
}
