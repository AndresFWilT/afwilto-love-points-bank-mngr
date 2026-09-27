package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/service"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/testsupport"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// earnFor abona `amount` a favor de `to` para que tenga saldo que canjear.
func earnFor(t *testing.T, repo *testsupport.MemoryRepo, to, by domain.MemberID, amount int) {
	t.Helper()
	svc := newService(repo)
	// se abona en trozos de máximo 5 (RD-1) hasta juntar el monto pedido
	for amount > 0 {
		chunk := amount
		if chunk > 5 {
			chunk = 5
		}
		if _, err := svc.Award(context.Background(), in.AwardCommand{
			AwardedTo: to, AwardedBy: by, Amount: chunk,
			Reason: "porque hizo algo digno de puntitos", OccurredAt: ayer,
		}); err != nil {
			t.Fatal(err)
		}
		amount -= chunk
	}
}

func setupRedeem(t *testing.T, available int, cost int) (*testsupport.MemoryRepo, *service.RedemptionService, domain.ItemID) {
	t.Helper()
	repo := seededRepo()
	earnFor(t, repo, memA, memB, available) // memA junta `available` puntitos
	cat := newCatalog(repo)
	item, err := cat.CreateItem(context.Background(), in.CreateItemCommand{Name: "Noche de película", Cost: cost})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewRedemption(repo.RedemptionRepo(), repo.CatalogRepo(), testsupport.FixedClock{T: now}, &testsupport.SeqIDs{})
	return repo, svc, item.ID
}

func availableOf(t *testing.T, repo *testsupport.MemoryRepo, m domain.MemberID) int {
	t.Helper()
	bals, _ := repo.Balances(context.Background())
	for _, b := range bals {
		if b.MemberID == m {
			return b.PointsAvailable
		}
	}
	t.Fatalf("sin saldo para %s", m)
	return 0
}

// spec 007 CA-1
func TestRedeem_HappyPath(t *testing.T) {
	repo, svc, itemID := setupRedeem(t, 10, 8)
	r, err := svc.Redeem(context.Background(), in.RedeemCommand{
		RedeemedBy: memA, CatalogItemID: itemID, Amount: 8, Note: "porque nos lo merecemos hoy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Cost != 8 || r.Status != domain.StatusActive {
		t.Fatalf("canje inesperado: %+v", r)
	}
	if got := availableOf(t, repo, memA); got != 2 {
		t.Fatalf("disponible debería bajar a 2, obtuve %d", got)
	}
}

// spec 007 CA-2: sin sobregiro
func TestRedeem_InsufficientBalance(t *testing.T) {
	_, svc, itemID := setupRedeem(t, 10, 8)
	_, err := svc.Redeem(context.Background(), in.RedeemCommand{
		RedeemedBy: memA, CatalogItemID: itemID, Amount: 12, Note: "porque quiero gastar de más",
	})
	if !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Fatalf("esperaba ErrInsufficientBalance, obtuve %v", err)
	}
}

// spec 007 CA-3, CA-9
func TestRedeem_DomainValidation(t *testing.T) {
	_, svc, itemID := setupRedeem(t, 10, 8)
	if _, err := svc.Redeem(context.Background(), in.RedeemCommand{RedeemedBy: memA, CatalogItemID: itemID, Amount: 0, Note: "porque nos lo merecemos hoy"}); !errors.Is(err, domain.ErrAmountNotPositive) {
		t.Fatalf("esperaba ErrAmountNotPositive, obtuve %v", err)
	}
	if _, err := svc.Redeem(context.Background(), in.RedeemCommand{RedeemedBy: memA, CatalogItemID: itemID, Amount: 3, Note: "corto"}); !errors.Is(err, domain.ErrNoteTooShort) {
		t.Fatalf("esperaba ErrNoteTooShort, obtuve %v", err)
	}
}

// spec 007 CA-4: plan inactivo
func TestRedeem_InactiveItem(t *testing.T) {
	repo, svc, itemID := setupRedeem(t, 10, 8)
	cat := newCatalog(repo)
	_, _ = cat.EditItem(context.Background(), in.EditItemCommand{ItemID: itemID, Name: "Noche de película", Cost: 8, IsActive: false})

	_, err := svc.Redeem(context.Background(), in.RedeemCommand{RedeemedBy: memA, CatalogItemID: itemID, Amount: 8, Note: "porque nos lo merecemos hoy"})
	if !errors.Is(err, domain.ErrItemInactive) {
		t.Fatalf("esperaba ErrItemInactive, obtuve %v", err)
	}
}

// spec 007 CA-5: híbrido, congela el monto gastado
func TestRedeem_Hybrid(t *testing.T) {
	repo, svc, itemID := setupRedeem(t, 10, 8)
	r, _ := svc.Redeem(context.Background(), in.RedeemCommand{RedeemedBy: memA, CatalogItemID: itemID, Amount: 6, Note: "porque solo queremos algo chico"})
	if r.Cost != 6 {
		t.Fatalf("debería congelar 6, obtuve %d", r.Cost)
	}
	if got := availableOf(t, repo, memA); got != 4 {
		t.Fatalf("disponible debería ser 4, obtuve %d", got)
	}
}

// spec 007 CA-6, CA-7
func TestRedeem_Void(t *testing.T) {
	repo, svc, itemID := setupRedeem(t, 10, 8)
	r, _ := svc.Redeem(context.Background(), in.RedeemCommand{RedeemedBy: memA, CatalogItemID: itemID, Amount: 8, Note: "porque nos lo merecemos hoy"})

	if _, err := svc.Void(context.Background(), r.ID, memB); !errors.Is(err, domain.ErrNotTheRedeemer) {
		t.Fatalf("otro no puede anular: esperaba ErrNotTheRedeemer, obtuve %v", err)
	}
	if _, err := svc.Void(context.Background(), r.ID, memA); err != nil {
		t.Fatal(err)
	}
	if got := availableOf(t, repo, memA); got != 10 {
		t.Fatalf("tras anular, el disponible vuelve a 10, obtuve %d", got)
	}
}

// spec 007 CA-8: concurrencia — dos canjes que juntos exceden el saldo; uno falla.
func TestRedeem_ConcurrentNoOverdraft(t *testing.T) {
	repo, svc, itemID := setupRedeem(t, 10, 8)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.Redeem(context.Background(), in.RedeemCommand{
				RedeemedBy: memA, CatalogItemID: itemID, Amount: 8, Note: "porque los dos queremos canjear ya",
			})
		}(i)
	}
	wg.Wait()

	failures := 0
	for _, err := range errs {
		if errors.Is(err, domain.ErrInsufficientBalance) {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("exactamente un canje debía fallar por saldo, fallaron %d", failures)
	}
	if got := availableOf(t, repo, memA); got < 0 {
		t.Fatalf("el saldo nunca debe quedar negativo, quedó en %d", got)
	}
}
