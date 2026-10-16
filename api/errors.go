package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/vlastrog/greenh/core"
)

// conflictError carries a user-facing 409 message.
type conflictError struct{ msg string }

func (e conflictError) Error() string { return e.msg }

func (e conflictError) Unwrap() error { return core.ErrConflict }

func writeError(w http.ResponseWriter, status int, body Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// requestError handles requests the generated code could not parse.
func (s *server) requestError(w http.ResponseWriter, _ *http.Request, err error) {
	writeError(w, http.StatusBadRequest, Error{Code: ErrorCodeInvalidRequest, Message: err.Error()})
}

// responseError maps a handler error to its status and Error body.
func (s *server) responseError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *core.ValidationError
	switch {
	case errors.As(err, &verr):
		details := make([]FieldError, len(verr.Fields))
		for i, f := range verr.Fields {
			details[i] = FieldError{Field: f.Field, Message: f.Message}
		}
		writeError(w, http.StatusBadRequest, Error{Code: ErrorCodeInvalidRequest, Message: verr.Error(), Details: &details})
	case errors.Is(err, core.ErrNotFound):
		writeError(w, http.StatusNotFound, Error{Code: ErrorCodeNotFound, Message: err.Error()})
	case errors.Is(err, core.ErrConflict):
		writeError(w, http.StatusConflict, Error{Code: ErrorCodeConflict, Message: err.Error()})
	default:
		s.d.Log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		writeError(w, http.StatusInternalServerError, Error{Code: ErrorCodeInternal, Message: "internal error"})
	}
}

func (s *server) routeNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, Error{Code: ErrorCodeNotFound, Message: fmt.Sprintf("no route for %s %s", r.Method, r.URL.Path)})
}

// notFound names the missing resource, e.g. "pot 7 not found".
func notFound(err error, kind string, id any) error {
	if errors.Is(err, core.ErrNotFound) {
		return fmt.Errorf("%s %v %w", kind, id, err)
	}
	return err
}

func validationError(fields ...core.FieldError) error {
	return &core.ValidationError{Fields: fields}
}
