package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/service"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/testsupport"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

func newCatalog(repo *testsupport.MemoryRepo) *service.CatalogService {
	return service.NewCatalog(repo.CatalogRepo(), testsupport.FixedClock{T: now}, &testsupport.SeqIDs{})
}

// spec 006 CA-1
func TestCreateItem_HappyPath(t *testing.T) {
	svc := newCatalog(seededRepo())
	item, err := svc.CreateItem(context.Background(), in.CreateItemCommand{Name: "Noche de película", Cost: 8})
	if err != nil {
		t.Fatal(err)
	}
	if item.Tier() != domain.TierMedio || !item.IsActive || item.ID == "" {
		t.Fatalf("plan inesperado: %+v", item)
	}
}

// spec 006 CA-2, CA-3
func TestCreateItem_RejectsInvalid(t *testing.T) {
	svc := newCatalog(seededRepo())
	if _, err := svc.CreateItem(context.Background(), in.CreateItemCommand{Name: "Noche de película", Cost: 0}); !errors.Is(err, domain.ErrCostNotPositive) {
		t.Fatalf("esperaba ErrCostNotPositive, obtuve %v", err)
	}
	if _, err := svc.CreateItem(context.Background(), in.CreateItemCommand{Name: "ab", Cost: 8}); !errors.Is(err, domain.ErrNameTooShort) {
		t.Fatalf("esperaba ErrNameTooShort, obtuve %v", err)
	}
}

// spec 006 CA-5: desactivar lo saca del listado de activos
func TestEditItem_Deactivate(t *testing.T) {
	repo := seededRepo()
	svc := newCatalog(repo)
	item, _ := svc.CreateItem(context.Background(), in.CreateItemCommand{Name: "Noche de película", Cost: 8})

	_, err := svc.EditItem(context.Background(), in.EditItemCommand{
		ItemID: item.ID, Name: item.Name, Cost: item.Cost, IsActive: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	active, _ := svc.ListItems(context.Background(), true)
	if len(active) != 0 {
		t.Fatalf("un plan desactivado no debería aparecer entre los activos, hay %d", len(active))
	}
	all, _ := svc.ListItems(context.Background(), false)
	if len(all) != 1 {
		t.Fatalf("pero sigue existiendo en el listado completo, hay %d", len(all))
	}
}

// spec 006 CA-6: editar el costo cambia el tier
func TestEditItem_ChangesTier(t *testing.T) {
	repo := seededRepo()
	svc := newCatalog(repo)
	item, _ := svc.CreateItem(context.Background(), in.CreateItemCommand{Name: "Noche de película", Cost: 8})

	edited, _ := svc.EditItem(context.Background(), in.EditItemCommand{
		ItemID: item.ID, Name: item.Name, Cost: 60, IsActive: true,
	})
	if edited.Tier() != domain.TierAlto {
		t.Fatalf("cost 60 debería ser alto, obtuve %q", edited.Tier())
	}
}

func TestEditItem_NotFound(t *testing.T) {
	svc := newCatalog(seededRepo())
	_, err := svc.EditItem(context.Background(), in.EditItemCommand{ItemID: "nope", Name: "Noche de película", Cost: 8, IsActive: true})
	if !errors.Is(err, domain.ErrItemNotFound) {
		t.Fatalf("esperaba ErrItemNotFound, obtuve %v", err)
	}
}
