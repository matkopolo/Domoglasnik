package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"

	"github.com/matkopolo/NepremicninskiAgent/apps/real-estate-api/internal/identity"
)

const (
	maxAuthBody        = 16 << 10
	maxPreferencesBody = 64 << 10
)

type credentialsRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if !decodeStrictJSON(w, r, maxAuthBody, &request) {
		return
	}
	if !validRegistration(w, r, request) {
		return
	}
	session, err := s.identities.Register(r.Context(), request.Email,
		request.DisplayName, request.Password, r.UserAgent())
	if errors.Is(err, identity.ErrEmailExists) {
		writeError(w, r, http.StatusConflict, "email_exists",
			"an account with this email already exists")
		return
	}
	if err != nil {
		s.logger.Error("register user", "error", err)
		writeError(w, r, http.StatusInternalServerError, "registration_failed",
			"could not create account")
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var request credentialsRequest
	if !decodeStrictJSON(w, r, maxAuthBody, &request) {
		return
	}
	if strings.TrimSpace(request.Email) == "" || request.Password == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_credentials",
			"email and password are required")
		return
	}
	session, err := s.identities.Login(r.Context(), request.Email,
		request.Password, r.UserAgent())
	if errors.Is(err, identity.ErrInvalidCredentials) {
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials",
			"email or password is incorrect")
		return
	}
	if err != nil {
		s.logger.Error("login user", "error", err)
		writeError(w, r, http.StatusInternalServerError, "login_failed",
			"could not sign in")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.authenticatedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	_, token, ok := s.authenticatedUser(w, r)
	if !ok {
		return
	}
	if err := s.identities.Logout(r.Context(), token); err != nil {
		s.logger.Error("logout user", "error", err)
		writeError(w, r, http.StatusInternalServerError, "logout_failed",
			"could not sign out")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) getPreferences(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.authenticatedUser(w, r)
	if !ok {
		return
	}
	preferences, err := s.identities.Preferences(r.Context(), user.ID)
	if err != nil {
		s.logger.Error("load preferences", "error", err)
		writeError(w, r, http.StatusInternalServerError, "query_failed",
			"could not load preferences")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": preferences})
}

func (s *server) savePreferences(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.authenticatedUser(w, r)
	if !ok {
		return
	}
	var document map[string]json.RawMessage
	if !decodeStrictJSON(w, r, maxPreferencesBody, &document) {
		return
	}
	preferences, err := json.Marshal(document)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_preferences",
			"preferences must be a JSON object")
		return
	}
	saved, err := s.identities.SavePreferences(r.Context(), user.ID, preferences)
	if err != nil {
		s.logger.Error("save preferences", "error", err)
		writeError(w, r, http.StatusInternalServerError, "save_failed",
			"could not save preferences")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preferences": saved})
}

func (s *server) authenticatedUser(
	w http.ResponseWriter, r *http.Request,
) (identity.User, string, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") ||
		strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) == "" {
		writeError(w, r, http.StatusUnauthorized, "authentication_required",
			"a bearer token is required")
		return identity.User{}, "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	user, err := s.identities.Authenticate(r.Context(), token)
	if errors.Is(err, identity.ErrInvalidSession) {
		writeError(w, r, http.StatusUnauthorized, "invalid_session",
			"session is invalid or expired")
		return identity.User{}, "", false
	}
	if err != nil {
		s.logger.Error("authenticate request", "error", err)
		writeError(w, r, http.StatusInternalServerError, "authentication_failed",
			"could not authenticate request")
		return identity.User{}, "", false
	}
	return user, token, true
}

func validRegistration(w http.ResponseWriter, r *http.Request, request credentialsRequest) bool {
	address, err := mail.ParseAddress(strings.TrimSpace(request.Email))
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(request.Email)) {
		writeError(w, r, http.StatusBadRequest, "invalid_email",
			"email must be a valid address")
		return false
	}
	if len(request.Password) < 12 || len(request.Password) > 72 {
		writeError(w, r, http.StatusBadRequest, "invalid_password",
			"password must contain between 12 and 72 characters")
		return false
	}
	name := strings.TrimSpace(request.DisplayName)
	if len(name) < 1 || len(name) > 100 {
		writeError(w, r, http.StatusBadRequest, "invalid_display_name",
			"displayName must contain between 1 and 100 characters")
		return false
	}
	return true
}

func decodeStrictJSON(
	w http.ResponseWriter, r *http.Request, maximum int64, destination any,
) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximum))
	if err != nil {
		writeError(w, r, http.StatusRequestEntityTooLarge, "request_too_large",
			"request body is too large")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json",
			"request body must contain valid JSON")
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		writeError(w, r, http.StatusBadRequest, "invalid_json",
			"request body must contain one JSON value")
		return false
	}
	return true
}
