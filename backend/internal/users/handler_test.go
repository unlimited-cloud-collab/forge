package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"forge/internal/database"
	"time"

	"forge/internal/sessions"
)

type handlerUserRepository struct {
	users map[string]database.User
}

func newHandlerUserRepository() *handlerUserRepository {
	return &handlerUserRepository{
		users: make(map[string]database.User),
	}
}

func (r *handlerUserRepository) Create(
	_ context.Context,
	user database.User,
) error {
	if _, exists := r.users[user.Email]; exists {
		return errors.New("duplicate email")
	}

	r.users[user.Email] = user

	return nil
}

func (r *handlerUserRepository) GetByEmail(
	_ context.Context,
	email string,
) (database.User, error) {
	user, exists := r.users[email]
	if !exists {
		return database.User{}, database.ErrUserNotFound
	}

	return user, nil
}

func TestRegisterHandlerCreatesUser(t *testing.T) {
	service := NewService(newHandlerUserRepository())
	handler := NewHandler(service, nil, false)

	body := bytes.NewBufferString(
		`{"email":"alice@example.com","password":"correct horse battery staple"}`,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		body,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}

	var response userResponse

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ID == uuid.Nil {
		t.Fatal("expected user ID")
	}

	if response.Email != "alice@example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"alice@example.com",
			response.Email,
		)
	}

	if bytes.Contains(
		recorder.Body.Bytes(),
		[]byte("password"),
	) {
		t.Fatal("response must not contain password data")
	}
}

func TestRegisterHandlerRejectsMalformedJSON(t *testing.T) {
	service := NewService(newHandlerUserRepository())
	handler := NewHandler(service, nil, false)

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		bytes.NewBufferString(`{"email":`),
	)

	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}
}

func TestRegisterHandlerRejectsUnknownFields(t *testing.T) {
	service := NewService(newHandlerUserRepository())
	handler := NewHandler(service, nil, false)

	request := httptest.NewRequest(
		http.MethodPost,
		"/users",
		bytes.NewBufferString(
			`{"email":"alice@example.com","password":"password","role":"admin"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}
}

func TestRegisterHandlerRejectsDuplicateEmail(t *testing.T) {
	repository := newHandlerUserRepository()
	service := NewService(repository)
	handler := NewHandler(service, nil, false)

	first := httptest.NewRequest(
		http.MethodPost,
		"/users",
		bytes.NewBufferString(
			`{"email":"alice@example.com","password":"password-one"}`,
		),
	)

	firstRecorder := httptest.NewRecorder()

	handler.Register(firstRecorder, first)

	if firstRecorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected first registration status %d, got %d",
			http.StatusCreated,
			firstRecorder.Code,
		)
	}

	second := httptest.NewRequest(
		http.MethodPost,
		"/users",
		bytes.NewBufferString(
			`{"email":"alice@example.com","password":"password-two"}`,
		),
	)

	secondRecorder := httptest.NewRecorder()

	handler.Register(secondRecorder, second)

	if secondRecorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusConflict,
			secondRecorder.Code,
		)
	}
}

func TestRegisterHandlerRejectsWrongMethod(t *testing.T) {
	service := NewService(newHandlerUserRepository())
	handler := NewHandler(service, nil, false)

	request := httptest.NewRequest(
		http.MethodGet,
		"/users",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Register(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}
}

func TestLoginHandlerAuthenticatesUser(t *testing.T) {
	repository := newHandlerUserRepository()
	service := NewService(repository)
	sessionCreator := &fakeSessionCreator{
	session: sessions.CreatedSession{
		ID:        uuid.New(),
		Token:     "test-session-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		},
	}

	handler := NewHandler(
		service,
		sessionCreator,
		false,
	)

	_, err := service.Register(
		context.Background(),
		"alice@example.com",
		"correct password",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		bytes.NewBufferString(
			`{"email":"alice@example.com","password":"correct password"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	var response userResponse

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Email != "alice@example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"alice@example.com",
			response.Email,
		)
	}

	if bytes.Contains(
		recorder.Body.Bytes(),
		[]byte("password"),
	) {
		t.Fatal("response must not contain password data")
	}
}

func TestLoginHandlerRejectsInvalidCredentials(t *testing.T) {
	repository := newHandlerUserRepository()
	service := NewService(repository)
	handler := NewHandler(service, nil, false)

	_, err := service.Register(
		context.Background(),
		"alice@example.com",
		"correct password",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		bytes.NewBufferString(
			`{"email":"alice@example.com","password":"wrong password"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}
}

func TestLoginHandlerDoesNotRevealUnknownUser(t *testing.T) {
	repository := newHandlerUserRepository()
	service := NewService(repository)
	handler := NewHandler(service, nil, false)

	request := httptest.NewRequest(
		http.MethodPost,
		"/login",
		bytes.NewBufferString(
			`{"email":"missing@example.com","password":"wrong password"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.Login(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			recorder.Code,
		)
	}

	var response errorResponse

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "invalid credentials" {
		t.Fatalf(
			"expected error %q, got %q",
			"invalid credentials",
			response.Error,
		)
	}
}

type fakeSessionCreator struct {
	session sessions.CreatedSession
	err     error
	calls   int
}

func (f *fakeSessionCreator) Create(
	_ context.Context,
	userID uuid.UUID,
) (sessions.CreatedSession, error) {
	f.calls++

	if f.err != nil {
		return sessions.CreatedSession{}, f.err
	}

	f.session.UserID = userID

	return f.session, nil
}