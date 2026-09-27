package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// ItemID identifica un plan del catálogo.
type ItemID string

func (i ItemID) String() string { return string(i) }

// Tier es el nivel de un plan. NO se ingresa: se deriva del costo (RD-CAT-3).
type Tier string

const (
	TierBaratito Tier = "baratito"
	TierMedio    Tier = "medio"
	TierAlto     Tier = "alto"
)

const (
	MinCost          = 1  // RD-CAT-1
	MinItemNameRune  = 3  // RD-CAT-2
	tierMedioMaxCost = 50 // ≤5 baratito, 6..50 medio, ≥51 alto
	tierBaratitoMax  = 5
)

// TierFor es la única fuente de la derivación en el código; la base la replica en una
// columna generada. Si cambia un umbral, cambia aquí y en una nueva migración.
func TierFor(cost int) Tier {
	switch {
	case cost <= tierBaratitoMax:
		return TierBaratito
	case cost <= tierMedioMaxCost:
		return TierMedio
	default:
		return TierAlto
	}
}

// CatalogItem es un plan por el que se pueden canjear puntitos.
type CatalogItem struct {
	ID          ItemID
	Name        string
	Description string
	Cost        int
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Tier deriva el nivel del costo actual del plan.
func (c CatalogItem) Tier() Tier { return TierFor(c.Cost) }

func validateItem(name string, cost int) error {
	if cost < MinCost {
		return ErrCostNotPositive // RD-CAT-1
	}
	if utf8.RuneCountInString(strings.TrimSpace(name)) < MinItemNameRune {
		return ErrNameTooShort // RD-CAT-2
	}
	return nil
}

// NewCatalogItem es el único camino para crear un plan válido.
func NewCatalogItem(id ItemID, name, description string, cost int, now time.Time) (CatalogItem, error) {
	if err := validateItem(name, cost); err != nil {
		return CatalogItem{}, err
	}
	return CatalogItem{
		ID:          id,
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Cost:        cost,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Edit devuelve una copia con nombre, descripción, costo y estado nuevos (RD-CAT-4/5).
func (c CatalogItem) Edit(name, description string, cost int, isActive bool, now time.Time) (CatalogItem, error) {
	if err := validateItem(name, cost); err != nil {
		return CatalogItem{}, err
	}
	edited := c
	edited.Name = strings.TrimSpace(name)
	edited.Description = strings.TrimSpace(description)
	edited.Cost = cost
	edited.IsActive = isActive
	edited.UpdatedAt = now
	return edited, nil
}

// RehydrateCatalogItem reconstruye un plan persistido sin revalidar invariantes.
func RehydrateCatalogItem(id ItemID, name, description string, cost int, isActive bool,
	createdAt, updatedAt time.Time) CatalogItem {
	return CatalogItem{
		ID:          id,
		Name:        name,
		Description: description,
		Cost:        cost,
		IsActive:    isActive,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
