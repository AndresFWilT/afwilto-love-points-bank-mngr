package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

func activeItem(t *testing.T, cost int) domain.CatalogItem {
	t.Helper()
	item, err := domain.NewCatalogItem("plan-1", "Noche de película", "", cost, nowC)
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// RD-CAN-1
func TestNewRedemption_RejectsNonPositiveAmount(t *testing.T) {
	item := activeItem(t, 8)
	for _, amount := range []int{0, -1} {
		_, err := domain.NewRedemption("r1", memA, item, amount, "porque nos lo merecemos hoy", nowC)
		if !errors.Is(err, domain.ErrAmountNotPositive) {
			t.Fatalf("amount=%d: esperaba ErrAmountNotPositive, obtuve %v", amount, err)
		}
	}
}

// RD-CAN-9
func TestNewRedemption_RejectsShortNote(t *testing.T) {
	item := activeItem(t, 8)
	for _, note := range []string{"", "   ", "corto", "  hola  "} {
		_, err := domain.NewRedemption("r1", memA, item, 8, note, nowC)
		if !errors.Is(err, domain.ErrNoteTooShort) {
			t.Fatalf("note=%q: esperaba ErrNoteTooShort, obtuve %v", note, err)
		}
	}
}

// RD-CAN-3
func TestNewRedemption_RejectsInactiveItem(t *testing.T) {
	item := activeItem(t, 8)
	inactive, _ := item.Edit(item.Name, "", item.Cost, false, nowC)
	_, err := domain.NewRedemption("r1", memA, inactive, 8, "porque nos lo merecemos hoy", nowC)
	if !errors.Is(err, domain.ErrItemInactive) {
		t.Fatalf("esperaba ErrItemInactive, obtuve %v", err)
	}
}

// RD-CAN-4 y RD-CAN-5: congela el amount gastado (que puede diferir del costo del plan).
func TestNewRedemption_FreezesAmount(t *testing.T) {
	item := activeItem(t, 8)
	r, err := domain.NewRedemption("r1", memA, item, 6, "porque queremos algo pequeño hoy", nowC)
	if err != nil {
		t.Fatal(err)
	}
	if r.Cost != 6 {
		t.Fatalf("cost_at_redemption debería ser 6 (híbrido), obtuve %d", r.Cost)
	}
	if r.CatalogItemID != item.ID || r.MemberID != memA || r.Status != domain.StatusActive {
		t.Fatalf("canje mal construido: %+v", r)
	}
}

func TestNewRedemption_TrimsNote(t *testing.T) {
	item := activeItem(t, 8)
	r, _ := domain.NewRedemption("r1", memA, item, 8, "   porque nos lo merecemos hoy   ", nowC)
	if r.Note != "porque nos lo merecemos hoy" {
		t.Fatalf("la nota debería quedar recortada, obtuve %q", r.Note)
	}
}

// RD-CAN-6, RD-CAN-7
func TestRedemption_Void(t *testing.T) {
	item := activeItem(t, 8)
	r, _ := domain.NewRedemption("r1", memA, item, 8, "porque nos lo merecemos hoy", nowC)

	t.Run("no es quien canjeó", func(t *testing.T) {
		if _, err := r.Void(nowC, memB); !errors.Is(err, domain.ErrNotTheRedeemer) {
			t.Fatalf("esperaba ErrNotTheRedeemer, obtuve %v", err)
		}
	})
	t.Run("quien canjeó anula", func(t *testing.T) {
		v, err := r.Void(nowC, memA)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != domain.StatusVoided {
			t.Fatalf("esperaba voided, obtuve %v", v.Status)
		}
	})
	t.Run("doble anulación", func(t *testing.T) {
		v, _ := r.Void(nowC, memA)
		if _, err := v.Void(nowC, memA); !errors.Is(err, domain.ErrRedemptionVoided) {
			t.Fatalf("esperaba ErrRedemptionVoided, obtuve %v", err)
		}
	})
}

func TestRehydrateRedemption(t *testing.T) {
	occurred := nowC.Add(-time.Hour)
	r := domain.RehydrateRedemption("r1", memA, "plan-1", 8, "porque nos lo merecemos hoy",
		domain.StatusActive, occurred, occurred)
	if r.Cost != 8 || r.MemberID != memA {
		t.Fatalf("rehidratación inesperada: %+v", r)
	}
}
