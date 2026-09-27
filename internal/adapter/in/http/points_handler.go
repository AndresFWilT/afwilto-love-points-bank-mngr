package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

var (
	errBadCursor  = errors.New("cursor inválido")
	errBadBody    = errors.New("cuerpo inválido")
	errBadMember  = errors.New("X-Member-Id inválido o ausente")
	errBadPageArg = errors.New("page_size inválido")
)

// Handler traduce HTTP ↔ casos de uso. No contiene ninguna regla de negocio.
type Handler struct {
	uc in.PointsUseCase
}

func NewHandler(uc in.PointsUseCase) *Handler { return &Handler{uc: uc} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// actor extrae el titular del header X-Member-Id (spec 000).
func actor(r *http.Request) domain.MemberID {
	return domain.MemberID(r.Header.Get("X-Member-Id"))
}

func (h *Handler) Award(w http.ResponseWriter, r *http.Request) {
	by := actor(r)
	if by == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	var body awardRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblemStatus(w, http.StatusBadRequest, errBadBody.Error(), "bad_body")
		return
	}

	entry, err := h.uc.Award(r.Context(), in.AwardCommand{
		AwardedTo:  domain.MemberID(body.AwardedTo),
		AwardedBy:  by,
		Amount:     body.Amount,
		Reason:     body.Reason,
		OccurredAt: body.OccurredAt,
	})
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEntryDTO(entry))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := in.LedgerFilter{}

	if v := q.Get("awarded_to"); v != "" {
		m := domain.MemberID(v)
		f.AwardedTo = &m
	}
	if v := q.Get("status"); v != "" {
		s := domain.EntryStatus(v)
		f.Status = &s
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeProblemStatus(w, http.StatusBadRequest, errBadPageArg.Error(), "bad_page_size")
			return
		}
		f.PageSize = n
	}
	if v := q.Get("cursor"); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			writeProblemStatus(w, http.StatusBadRequest, errBadCursor.Error(), "bad_cursor")
			return
		}
		f.After = c
	}

	page, err := h.uc.Ledger(r.Context(), f)
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toLedgerPageDTO(page))
}

func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	by := actor(r)
	if by == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	var body editRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblemStatus(w, http.StatusBadRequest, errBadBody.Error(), "bad_body")
		return
	}

	entry, err := h.uc.Edit(r.Context(), in.EditCommand{
		EntryID:    domain.EntryID(chi.URLParam(r, "id")),
		EditedBy:   by,
		Amount:     body.Amount,
		Reason:     body.Reason,
		OccurredAt: body.OccurredAt,
	})
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEntryDTO(entry))
}

func (h *Handler) Void(w http.ResponseWriter, r *http.Request) {
	by := actor(r)
	if by == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	entry, err := h.uc.Void(r.Context(), domain.EntryID(chi.URLParam(r, "id")), by)
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toEntryDTO(entry))
}

func (h *Handler) Balances(w http.ResponseWriter, r *http.Request) {
	bals, err := h.uc.Balances(r.Context())
	if err != nil {
		writeProblem(w, err)
		return
	}
	out := make([]balanceDTO, 0, len(bals))
	for _, b := range bals {
		out = append(out, toBalanceDTO(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	members, err := h.uc.Members(r.Context())
	if err != nil {
		writeProblem(w, err)
		return
	}
	out := make([]memberDTO, 0, len(members))
	for _, m := range members {
		out = append(out, toMemberDTO(m))
	}
	writeJSON(w, http.StatusOK, out)
}
