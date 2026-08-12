package users

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request registerRequest

	if err := decoder.Decode(&request); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)
		return
	}

	var extra any

	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"request body must contain a single JSON object",
		)
		return
	}

	user, err := h.service.Register(
		r.Context(),
		request.Email,
		request.Password,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			writeJSONError(w, http.StatusBadRequest, "invalid email")
		case errors.Is(err, ErrInvalidPassword):
			writeJSONError(w, http.StatusBadRequest, "invalid password")
		case errors.Is(err, ErrEmailTaken):
			writeJSONError(w, http.StatusConflict, "email already registered")
		default:
			writeJSONError(
				w,
				http.StatusInternalServerError,
				"internal server error",
			)
		}

		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		userResponse{
			ID:    user.ID,
			Email: user.Email,
		},
	)
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	writeJSON(
		w,
		status,
		errorResponse{
			Error: message,
		},
	)
}
