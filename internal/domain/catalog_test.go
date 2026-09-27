package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

var nowC = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

// RD-CAT-3: el nivel se deriva del costo. Bordes 5/6/50/51.
func TestTierFor(t *testing.T) {
	cases := []struct {
		cost int
		want domain.Tier
	}{
		{1, domain.TierBaratito},
		{5, domain.TierBaratito},
		{6, domain.TierMedio},
		{50, domain.TierMedio},
		{51, domain.TierAlto},
		{500, domain.TierAlto},
	}
	for _, tc := range cases {
		if got := domain.TierFor(tc.cost); got != tc.want {
			t.Fatalf("TierFor(%d) = %q, esperaba %q", tc.cost, got, tc.want)
		}
	}
}

// RD-CAT-1
func TestNewCatalogItem_RejectsNonPositiveCost(t *testing.T) {
	for _, cost := range []int{0, -1, -10} {
		if _, err := domain.NewCatalogItem("i1", "Noche de película", "", cost, nowC); !errors.Is(err, domain.ErrCostNotPositive) {
			t.Fatalf("cost=%d: esperaba ErrCostNotPositive, obtuve %v", cost, err)
		}
	}
}

// RD-CAT-2
func TestNewCatalogItem_RejectsShortName(t *testing.T) {
	for _, name := range []string{"", "  ", "ab", "  x "} {
		if _, err := domain.NewCatalogItem("i1", name, "", 8, nowC); !errors.Is(err, domain.ErrNameTooShort) {
			t.Fatalf("name=%q: esperaba ErrNameTooShort, obtuve %v", name, err)
		}
	}
}

func TestNewCatalogItem_HappyPath(t *testing.T) {
	item, err := domain.NewCatalogItem("i1", "  Noche de película  ", "con palomitas", 8, nowC)
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "Noche de película" {
		t.Fatalf("el nombre debería quedar recortado, obtuve %q", item.Name)
	}
	if item.Tier() != domain.TierMedio {
		t.Fatalf("cost 8 debería ser medio, obtuve %q", item.Tier())
	}
	if !item.IsActive {
		t.Fatal("un plan nace activo")
	}
}

// RD-CAT-4/5: editar cambia costo (y por tanto tier) y puede desactivar.
func TestCatalogItem_Edit(t *testing.T) {
	item, _ := domain.NewCatalogItem("i1", "Noche de película", "", 8, nowC)
	later := nowC.Add(time.Hour)

	edited, err := item.Edit("Escapada de finde", "dos días", 60, false, later)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Cost != 60 || edited.Tier() != domain.TierAlto {
		t.Fatalf("cost 60 debería ser alto, obtuve cost=%d tier=%q", edited.Cost, edited.Tier())
	}
	if edited.IsActive {
		t.Fatal("debería quedar desactivado")
	}
	if !edited.UpdatedAt.Equal(later) {
		t.Fatal("updated_at debería avanzar")
	}
}

func TestCatalogItem_EditRevalidates(t *testing.T) {
	item, _ := domain.NewCatalogItem("i1", "Noche de película", "", 8, nowC)
	if _, err := item.Edit("Noche de película", "", 0, true, nowC); !errors.Is(err, domain.ErrCostNotPositive) {
		t.Fatalf("editar a cost 0 debería fallar, obtuve %v", err)
	}
	if _, err := item.Edit("ab", "", 8, true, nowC); !errors.Is(err, domain.ErrNameTooShort) {
		t.Fatalf("editar a nombre corto debería fallar, obtuve %v", err)
	}
}

func TestRehydrateCatalogItem_NoRevalidation(t *testing.T) {
	// Un plan ya guardado se reconstruye sin revalidar (datos válidos por definición).
	item := domain.RehydrateCatalogItem("i1", "x", "", 8, true, nowC, nowC)
	if item.Cost != 8 || item.Tier() != domain.TierMedio {
		t.Fatalf("rehidratación inesperada: %+v", item)
	}
}
