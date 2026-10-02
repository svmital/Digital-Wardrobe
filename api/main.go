package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/svmital/digital-wardrobe/api/session"
	"github.com/svmital/digital-wardrobe/api/user"
	"github.com/svmital/digital-wardrobe/api/wardrobe"
	"golang.org/x/crypto/bcrypt"
)

func healthHandler(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(response, `{"status": "ok"}`)
}

type contextKey string

const userIDKey contextKey = "userID"

type wardrobeStore interface {
	List(string) ([]wardrobe.WardrobeItem, error)
	Create(string, wardrobe.WardrobeItem) (wardrobe.WardrobeItem, error)
	Update(string, string, wardrobe.WardrobeItem) (wardrobe.WardrobeItem, error)
	Delete(string, string) error
}

type userStore interface {
	Create(user.User) (user.User, error)
	FindByUsername(username string) (user.User, error)
}

type sessionStore interface {
	Create(session.Session) (session.Session, error)
	FindByTokenHash(string) (session.Session, error)
	DeleteByTokenHash(string) error
}

type registerRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func wardrobeHandler(store wardrobeStore) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		userID, ok := request.Context().Value(userIDKey).(string)
		if !ok {
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		id := strings.TrimPrefix(request.URL.Path, "/wardrobe/")
		if id == request.URL.Path {
			id = ""
		}

		if request.Method == http.MethodGet {
			items, err := store.List(userID)
			if err != nil {
				http.Error(response, "Could not load wardrobe", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(response).Encode(items)
			return
		}

		if request.Method == http.MethodPost {
			var item wardrobe.WardrobeItem
			if json.NewDecoder(request.Body).Decode(&item) != nil {
				http.Error(response, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if item.Name == "" || item.Category == "" || item.Color == "" {
				http.Error(response, "Name, category, and color are required", http.StatusBadRequest)
				return
			}

			item, err := store.Create(userID, item)
			if err != nil {
				http.Error(response, "Could not create wardrobe item", http.StatusInternalServerError)
				return
			}
			response.WriteHeader(http.StatusCreated)
			json.NewEncoder(response).Encode(item)
			return
		}

		if request.Method == http.MethodPut && id != "" {
			var item wardrobe.WardrobeItem
			if json.NewDecoder(request.Body).Decode(&item) != nil {
				http.Error(response, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if item.Name == "" || item.Category == "" || item.Color == "" {
				http.Error(response, "Name, category, and color are required", http.StatusBadRequest)
				return
			}

			item, err := store.Update(userID, id, item)
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(response, "Wardrobe item not found", http.StatusNotFound)
				return
			}
			if err != nil {
				http.Error(response, "Could not update wardrobe item", http.StatusInternalServerError)
				return
			}

			json.NewEncoder(response).Encode(item)
			return
		}

		if request.Method == http.MethodDelete && id != "" {
			err := store.Delete(userID, id)
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(response, "Wardrobe item not found", http.StatusNotFound)
				return
			}
			if err != nil {
				http.Error(response, "Could not delete wardrobe item", http.StatusInternalServerError)
				return
			}

			response.WriteHeader(http.StatusNoContent)
			return
		}

		http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func registerHandler(store userStore) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Method != http.MethodPost {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input registerRequest
		if json.NewDecoder(request.Body).Decode(&input) != nil {
			http.Error(response, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if input.Username == "" || input.Password == "" {
			http.Error(response, "Username and password are required", http.StatusBadRequest)
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword(
			[]byte(input.Password), bcrypt.DefaultCost,
		)
		if err != nil {
			http.Error(response, "Could not create user", http.StatusInternalServerError)
			return
		}

		newUser := user.User{
			Username:     input.Username,
			PasswordHash: string(passwordHash),
		}
		createdUser, err := store.Create(newUser)
		if err != nil {
			http.Error(response, "Could not create user", http.StatusInternalServerError)
			return
		}

		response.WriteHeader(http.StatusCreated)
		json.NewEncoder(response).Encode(createdUser)

	}
}

func loginHandler(users userStore, sessions sessionStore) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")

		if request.Method != http.MethodPost {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input loginRequest
		if json.NewDecoder(request.Body).Decode(&input) != nil {
			http.Error(response, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if input.Username == "" || input.Password == "" {
			http.Error(response, "Username and password are required", http.StatusBadRequest)
			return
		}

		user, err := users.FindByUsername(input.Username)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(response, "Invalid username or password", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(response, "Could not log in", http.StatusInternalServerError)
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password))
		if err != nil {
			http.Error(response, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		rawSessionToken := rand.Text()
		sessionTokenHash := sha256.Sum256([]byte(rawSessionToken))
		tokenHashString := hex.EncodeToString(sessionTokenHash[:])

		newSession := session.Session{
			TokenHash: tokenHashString,
			UserID:    user.ID,
		}
		createdSession, err := sessions.Create(newSession)
		if err != nil {
			http.Error(response, "Could not create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(response, &http.Cookie{
			Name:     "session",
			Value:    rawSessionToken,
			Path:     "/",
			Expires:  createdSession.ExpiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false, // Local HTTP development only
		})

		response.WriteHeader(http.StatusOK)
		json.NewEncoder(response).Encode(map[string]string{"message": "Login successful"})
	}
}

func logoutHandler(sessions sessionStore) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cookie, err := request.Cookie("session")
		if err != nil {
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenHash := sha256.Sum256([]byte(cookie.Value))
		tokenHashString := hex.EncodeToString(tokenHash[:])
		if err := sessions.DeleteByTokenHash(tokenHashString); err != nil {
			http.Error(response, "Could not log out", http.StatusInternalServerError)
			return
		}

		http.SetCookie(response, &http.Cookie{
			Name:     "session",
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false, // Local HTTP development only
		})
		response.WriteHeader(http.StatusNoContent)
	}
}

func authMiddleware(sessions sessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cookie, err := request.Cookie("session")
		if err != nil {
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sessionTokenHash := sha256.Sum256([]byte(cookie.Value))
		tokenHashString := hex.EncodeToString(sessionTokenHash[:])

		currentSession, err := sessions.FindByTokenHash(tokenHashString)
		if err != nil {
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(
			request.Context(),
			userIDKey,
			currentSession.UserID,
		)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func main() {
	databaseURL := "postgres://wardrobe:wardrobe@localhost:5432/wardrobe?sslmode=disable"
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to PostgreSQL")

	wardrobeStore := wardrobe.NewStore(db)
	userStore := user.NewStore(db)
	sessionStore := session.NewStore(db)

	http.HandleFunc("/health", healthHandler)
	http.Handle("/wardrobe", authMiddleware(sessionStore, wardrobeHandler(wardrobeStore)))
	http.Handle("/wardrobe/", authMiddleware(sessionStore, wardrobeHandler(wardrobeStore)))
	http.HandleFunc("/register", registerHandler(userStore))
	http.HandleFunc("/login", loginHandler(userStore, sessionStore))
	http.Handle("/logout", authMiddleware(sessionStore, logoutHandler(sessionStore)))

	fmt.Println("Server is running on port 8080...")
	http.ListenAndServe(":8080", nil)
}
