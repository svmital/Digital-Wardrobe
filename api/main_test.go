package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/svmital/digital-wardrobe/api/session"
	"github.com/svmital/digital-wardrobe/api/user"
	"github.com/svmital/digital-wardrobe/api/wardrobe"
	"golang.org/x/crypto/bcrypt"
)

type fakeStore struct {
	items []wardrobe.WardrobeItem
}

type fakeUserStore struct {
	user user.User
}

type fakeSessionStore struct {
	session session.Session
}

func (store *fakeSessionStore) Create(newSession session.Session) (session.Session, error) {
	newSession.CreatedAt = time.Now()
	newSession.ExpiresAt = newSession.CreatedAt.Add(24 * time.Hour)
	store.session = newSession
	return newSession, nil
}

func (store *fakeSessionStore) FindByTokenHash(tokenHash string) (session.Session, error) {
	if store.session.TokenHash != tokenHash {
		return session.Session{}, sql.ErrNoRows
	}
	return store.session, nil
}

func (store *fakeSessionStore) DeleteByTokenHash(tokenHash string) error {
	if store.session.TokenHash == tokenHash {
		store.session = session.Session{}
	}
	return nil
}

func (store *fakeUserStore) Create(newUser user.User) (user.User, error) {
	newUser.ID = "1"
	store.user = newUser
	return newUser, nil
}

func (store *fakeUserStore) FindByUsername(username string) (user.User, error) {
	if store.user.Username != username {
		return user.User{}, sql.ErrNoRows
	}
	return store.user, nil
}

func (store *fakeStore) List(userID string) ([]wardrobe.WardrobeItem, error) {
	var items []wardrobe.WardrobeItem
	for _, item := range store.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (store *fakeStore) Create(userID string, item wardrobe.WardrobeItem) (wardrobe.WardrobeItem, error) {
	item.UserID = userID
	store.items = append(store.items, item)
	return item, nil
}

func (store *fakeStore) Update(userID string, id string, item wardrobe.WardrobeItem) (wardrobe.WardrobeItem, error) {
	for index := range store.items {
		if store.items[index].ID == id && store.items[index].UserID == userID {
			item.ID = id
			item.UserID = userID
			store.items[index] = item
			return item, nil
		}
	}
	return wardrobe.WardrobeItem{}, sql.ErrNoRows
}

func (store *fakeStore) Delete(userID string, id string) error {
	for index := range store.items {
		if store.items[index].ID == id && store.items[index].UserID == userID {
			store.items = append(store.items[:index], store.items[index+1:]...)
			return nil
		}
	}
	return sql.ErrNoRows
}

func withUserID(request *http.Request, userID string) *http.Request {
	ctx := context.WithValue(request.Context(), userIDKey, userID)
	return request.WithContext(ctx)
}

func TestGetWardrobe(t *testing.T) {
	store := &fakeStore{}
	request := withUserID(httptest.NewRequest(http.MethodGet, "/wardrobe", nil), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
}

func TestPostWardrobe(t *testing.T) {
	store := &fakeStore{}
	body := `{"name":"Blue Jeans","category":"Bottoms","color":"Blue"}`
	request := withUserID(httptest.NewRequest(http.MethodPost, "/wardrobe", strings.NewReader(body)), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", response.Code)
	}
	if len(store.items) != 1 {
		t.Fatalf("expected 1 wardrobe item, got %d", len(store.items))
	}
}

func TestPostWardrobeInvalidJSON(t *testing.T) {
	store := &fakeStore{}
	request := withUserID(httptest.NewRequest(http.MethodPost, "/wardrobe", strings.NewReader(`not json`)), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestPostWardrobeMissingFields(t *testing.T) {
	store := &fakeStore{}
	request := withUserID(httptest.NewRequest(http.MethodPost, "/wardrobe", strings.NewReader(`{"name":"Blue Jeans"}`)), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestPutWardrobe(t *testing.T) {
	store := &fakeStore{items: []wardrobe.WardrobeItem{{ID: "1", UserID: "1", Name: "Blue Jeans"}}}
	body := `{"name":"Black Jeans","category":"Bottoms","color":"Black"}`
	request := withUserID(httptest.NewRequest(http.MethodPut, "/wardrobe/1", strings.NewReader(body)), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if store.items[0].Name != "Black Jeans" {
		t.Fatalf("expected updated name, got %q", store.items[0].Name)
	}
}

func TestDeleteWardrobe(t *testing.T) {
	store := &fakeStore{items: []wardrobe.WardrobeItem{{ID: "1", UserID: "1", Name: "Blue Jeans"}}}
	request := withUserID(httptest.NewRequest(http.MethodDelete, "/wardrobe/1", nil), "1")
	response := httptest.NewRecorder()

	wardrobeHandler(store)(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", response.Code)
	}
	if len(store.items) != 0 {
		t.Fatalf("expected wardrobe to be empty, got %d items", len(store.items))
	}
}

func TestRegisterUser(t *testing.T) {
	store := &fakeUserStore{}
	body := `{"username":"shubham","password":"testpassword"}`
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	response := httptest.NewRecorder()

	registerHandler(store)(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", response.Code)
	}
	if bcrypt.CompareHashAndPassword([]byte(store.user.PasswordHash), []byte("testpassword")) != nil {
		t.Fatal("password was not hashed correctly")
	}

	var responseBody map[string]any
	if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
		t.Fatal(err)
	}
	if _, exists := responseBody["password_hash"]; exists {
		t.Fatal("response should not include password_hash")
	}
}

func TestRegisterUserInvalidJSON(t *testing.T) {
	store := &fakeUserStore{}
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`not json`))
	response := httptest.NewRecorder()

	registerHandler(store)(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestRegisterUserMissingFields(t *testing.T) {
	store := &fakeUserStore{}
	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"username":"shubham"}`))
	response := httptest.NewRecorder()

	registerHandler(store)(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

func TestLoginUser(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("testpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	users := &fakeUserStore{user: user.User{
		ID:           "1",
		Username:     "shubham",
		PasswordHash: string(passwordHash),
	}}
	sessions := &fakeSessionStore{}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"username":"shubham","password":"testpassword"}`,
	))
	response := httptest.NewRecorder()

	loginHandler(users, sessions)(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "session" || !cookie.HttpOnly {
		t.Fatal("expected an HttpOnly session cookie")
	}

	hash := sha256.Sum256([]byte(cookie.Value))
	if sessions.session.TokenHash != hex.EncodeToString(hash[:]) {
		t.Fatal("stored session hash does not match the cookie token")
	}
}

func TestLoginUserWrongPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}

	users := &fakeUserStore{user: user.User{
		Username:     "shubham",
		PasswordHash: string(passwordHash),
	}}
	sessions := &fakeSessionStore{}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"username":"shubham","password":"wrongpassword"}`,
	))
	response := httptest.NewRecorder()

	loginHandler(users, sessions)(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
	if sessions.session.TokenHash != "" {
		t.Fatal("session should not be created for an incorrect password")
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("cookie should not be created for an incorrect password")
	}
}

func TestLoginUserUnknownUsername(t *testing.T) {
	users := &fakeUserStore{}
	sessions := &fakeSessionStore{}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(
		`{"username":"unknown","password":"testpassword"}`,
	))
	response := httptest.NewRecorder()

	loginHandler(users, sessions)(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
	if sessions.session.TokenHash != "" {
		t.Fatal("session should not be created for an unknown username")
	}
}

func TestAuthMiddlewareMissingCookie(t *testing.T) {
	sessions := &fakeSessionStore{}
	nextCalled := false
	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		nextCalled = true
	})
	request := httptest.NewRequest(http.MethodGet, "/wardrobe", nil)
	response := httptest.NewRecorder()

	authMiddleware(sessions, next).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}
	if nextCalled {
		t.Fatal("protected handler should not be called without a session cookie")
	}
}

func TestAuthMiddlewareValidCookie(t *testing.T) {
	rawToken := "test-session-token"
	hash := sha256.Sum256([]byte(rawToken))
	sessions := &fakeSessionStore{session: session.Session{
		TokenHash: hex.EncodeToString(hash[:]),
		UserID:    "7",
	}}

	var receivedUserID string
	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		receivedUserID, _ = request.Context().Value(userIDKey).(string)
		response.WriteHeader(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/wardrobe", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	response := httptest.NewRecorder()

	authMiddleware(sessions, next).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}
	if receivedUserID != "7" {
		t.Fatalf("expected user ID 7, got %q", receivedUserID)
	}
}

func TestLogoutUser(t *testing.T) {
	rawToken := "test-session-token"
	hash := sha256.Sum256([]byte(rawToken))
	sessions := &fakeSessionStore{session: session.Session{
		TokenHash: hex.EncodeToString(hash[:]),
		UserID:    "7",
	}}
	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: rawToken})
	response := httptest.NewRecorder()

	logoutHandler(sessions)(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", response.Code)
	}
	if sessions.session.TokenHash != "" {
		t.Fatal("session should be deleted")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatal("session cookie should be deleted")
	}
}
