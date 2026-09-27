package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// CatalogHandler traduce HTTP ↔ casos de uso del catálogo (spec 006).
type CatalogHandler struct {
	uc in.CatalogUseCase
}

func NewCatalogHandler(uc in.CatalogUseCase) *CatalogHandler { return &CatalogHandler{uc: uc} }

func (h *CatalogHandler) Create(w http.ResponseWriter, r *http.Request) {
	if actor(r) == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	var body createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblemStatus(w, http.StatusBadRequest, errBadBody.Error(), "bad_body")
		return
	}
	item, err := h.uc.CreateItem(r.Context(), in.CreateItemCommand{
		Name: body.Name, Description: body.Description, Cost: body.Cost,
	})
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCatalogItemDTO(item))
}

func (h *CatalogHandler) List(w http.ResponseWriter, r *http.Request) {
	onlyActive := r.URL.Query().Get("active") == "true"
	items, err := h.uc.ListItems(r.Context(), onlyActive)
	if err != nil {
		writeProblem(w, err)
		return
	}
	out := make([]catalogItemDTO, 0, len(items))
	for _, i := range items {
		out = append(out, toCatalogItemDTO(i))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *CatalogHandler) Edit(w http.ResponseWriter, r *http.Request) {
	if actor(r) == "" {
		writeProblemStatus(w, http.StatusUnauthorized, errBadMember.Error(), "missing_member")
		return
	}
	var body editItemRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblemStatus(w, http.StatusBadRequest, errBadBody.Error(), "bad_body")
		return
	}
	item, err := h.uc.EditItem(r.Context(), in.EditItemCommand{
		ItemID:      domain.ItemID(chi.URLParam(r, "id")),
		Name:        body.Name,
		Description: body.Description,
		Cost:        body.Cost,
		IsActive:    body.IsActive,
	})
	if err != nil {
		writeProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toCatalogItemDTO(item))
}
