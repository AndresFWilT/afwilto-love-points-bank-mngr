package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// problem es la representación RFC 7807 de un error.
type problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code,omitempty"`
}

// statusByError mapea cada error de dominio a su status HTTP. Es el ÚNICO lugar del
// proyecto donde una regla de negocio se traduce a un código de transporte.
var statusByError = map[error]int{
	domain.ErrReasonTooShort:   http.StatusUnprocessableEntity,
	domain.ErrAmountOutOfRange: http.StatusUnprocessableEntity,
	domain.ErrSelfAward:        http.StatusUnprocessableEntity,
	domain.ErrFutureDate:       http.StatusUnprocessableEntity,
	domain.ErrNotTheAwarder:    http.StatusForbidden,
	domain.ErrEntryVoided:      http.StatusConflict,
	domain.ErrEntryNotFound:    http.StatusNotFound,
	domain.ErrMemberNotFound:   http.StatusNotFound,

	// Fase 2 — catálogo y canjes
	domain.ErrCostNotPositive:     http.StatusUnprocessableEntity,
	domain.ErrNameTooShort:        http.StatusUnprocessableEntity,
	domain.ErrAmountNotPositive:   http.StatusUnprocessableEntity,
	domain.ErrNoteTooShort:        http.StatusUnprocessableEntity,
	domain.ErrInsufficientBalance: http.StatusUnprocessableEntity,
	domain.ErrItemInactive:        http.StatusConflict,
	domain.ErrRedemptionVoided:    http.StatusConflict,
	domain.ErrNotTheRedeemer:      http.StatusForbidden,
	domain.ErrItemNotFound:        http.StatusNotFound,
	domain.ErrRedemptionNotFound:  http.StatusNotFound,
}

func statusFor(err error) int {
	for domainErr, status := range statusByError {
		if errors.Is(err, domainErr) {
			return status
		}
	}
	return http.StatusInternalServerError
}

// writeProblem serializa un error de dominio como application/problem+json.
func writeProblem(w http.ResponseWriter, err error) {
	status := statusFor(err)
	writeProblemStatus(w, status, err.Error(), domain.Code(err))
}

func writeProblemStatus(w http.ResponseWriter, status int, detail, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	})
}
