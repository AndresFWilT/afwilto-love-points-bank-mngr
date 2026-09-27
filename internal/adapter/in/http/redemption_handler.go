package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// RedemptionHandler traduce HTTP ↔ casos de uso de canje (spec 007).
type RedemptionHandler struct {
	uc in.RedemptionUseCase
}

func NewRedemptionHandler(uc in.RedemptionUseCase) *RedemptionHandler {
	return &RedemptionHandler{uc: uc}
}

func (h *RedemptionHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	by := actor(r)
	if by == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	var body redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblemStatus(w, http.StatusBadRequest, errBadBody.Error(), "bad_body")
		return
	}
	red, err := h.uc.Redeem(r.Context(), in.RedeemCommand{
		RedeemedBy:    by,
		CatalogItemID: domain.ItemID(body.CatalogItemID),
		Amount:        body.Amount,
		Note:          body.Note,
	})
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRedemptionDTO(red))
}

func (h *RedemptionHandler) List(w http.ResponseWriter, r *http.Request) {
	f := in.RedemptionFilter{}
	if v := r.URL.Query().Get("member_id"); v != "" {
		m := domain.MemberID(v)
		f.MemberID = &m
	}
	reds, err := h.uc.List(r.Context(), f)
	if err != nil {
		writeProblem(w, err)
		return
	}
	out := make([]redemptionDTO, 0, len(reds))
	for _, red := range reds {
		out = append(out, toRedemptionDTO(red))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RedemptionHandler) Void(w http.ResponseWriter, r *http.Request) {
	by := actor(r)
	if by == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	red, err := h.uc.Void(r.Context(), domain.RedemptionID(chi.URLParam(r, "id")), by)
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRedemptionDTO(red))
}
