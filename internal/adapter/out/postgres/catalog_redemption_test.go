//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/adapter/out/postgres"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

func saveItem(t *testing.T, repo *postgres.CatalogRepository, cost int) domain.CatalogItem {
	t.Helper()
	item, err := domain.NewCatalogItem(domain.ItemID(randUUID()), "Noche de película", "", cost, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	return item
}

// La columna generada `tier` refleja el costo en los bordes.
func TestCatalog_GeneratedTier(t *testing.T) {
	truncate(t)
	repo := postgres.NewCatalogRepository(postgres.NewDB(testPool))

	for _, tc := range []struct {
		cost int
		want string
	}{{5, "baratito"}, {6, "medio"}, {50, "medio"}, {51, "alto"}} {
		item := saveItem(t, repo, tc.cost)
		var tier string
		if err := testPool.QueryRow(context.Background(),
			`SELECT tier FROM catalog_items WHERE id=$1`, item.ID.String()).Scan(&tier); err != nil {
			t.Fatal(err)
		}
		if tier != tc.want {
			t.Fatalf("cost %d: tier en la base = %q, esperaba %q", tc.cost, tier, tc.want)
		}
	}
}

// Editar el costo de un plan no cambia el precio congelado de canjes viejos (RD-CAN-4).
func TestRedemption_FreezesCost(t *testing.T) {
	truncate(t)
	catRepo := postgres.NewCatalogRepository(postgres.NewDB(testPool))
	redRepo := postgres.NewRedemptionRepository(postgres.NewDB(testPool))
	entryRepo := postgres.NewEntryRepository(postgres.NewDB(testPool))

	// memA junta saldo
	for i := 0; i < 3; i++ {
		e := newEntry(t, ids.mk(), 5, now.Add(-time.Hour))
		_ = entryRepo.Save(context.Background(), e)
	}
	item := saveItem(t, catRepo, 8)

	red, err := domain.NewRedemption(domain.RedemptionID(randUUID()), memA, item, 8, "porque nos lo merecemos hoy", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := redRepo.RedeemTx(context.Background(), red); err != nil {
		t.Fatal(err)
	}

	// sube el costo del plan
	edited, _ := item.Edit(item.Name, "", 20, true, now)
	_ = catRepo.Update(context.Background(), edited)

	got, _ := redRepo.FindByID(context.Background(), red.ID)
	if got.Cost != 8 {
		t.Fatalf("el canje viejo debía conservar cost 8, obtuve %d", got.Cost)
	}
}

// La vista de saldos resta los canjes activos y los devuelve al anular.
func TestBalanceView_AvailableWithRedemptions(t *testing.T) {
	truncate(t)
	catRepo := postgres.NewCatalogRepository(postgres.NewDB(testPool))
	redRepo := postgres.NewRedemptionRepository(postgres.NewDB(testPool))
	entryRepo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	memberRepo := postgres.NewMemberRepository(postgres.NewDB(testPool))

	for i := 0; i < 2; i++ { // 10 ganados
		_ = entryRepo.Save(context.Background(), newEntry(t, ids.mk(), 5, now.Add(-time.Hour)))
	}
	item := saveItem(t, catRepo, 8)
	red, _ := domain.NewRedemption(domain.RedemptionID(randUUID()), memA, item, 3, "porque nos lo merecemos hoy", now)
	_ = redRepo.RedeemTx(context.Background(), red)

	availA := func() (int, int, int) {
		bals, _ := memberRepo.Balances(context.Background())
		for _, b := range bals {
			if b.MemberID == memA {
				return b.PointsEarned, b.PointsRedeemed, b.PointsAvailable
			}
		}
		t.Fatal("sin saldo para A")
		return 0, 0, 0
	}

	earned, redeemed, available := availA()
	if earned != 10 || redeemed != 3 || available != 7 {
		t.Fatalf("esperaba 10/3/7, obtuve %d/%d/%d", earned, redeemed, available)
	}

	// anular devuelve los puntos
	voided, _ := red.Void(now, memA)
	_ = redRepo.UpdateStatus(context.Background(), voided)
	_, redeemed2, available2 := availA()
	if redeemed2 != 0 || available2 != 10 {
		t.Fatalf("tras anular esperaba 0 canjeado / 10 disponible, obtuve %d/%d", redeemed2, available2)
	}
}

// RD-CAN-2 bajo concurrencia: dos canjes simultáneos, uno debe fallar; saldo no negativo.
func TestRedemption_ConcurrentNoOverdraft(t *testing.T) {
	truncate(t)
	catRepo := postgres.NewCatalogRepository(postgres.NewDB(testPool))
	redRepo := postgres.NewRedemptionRepository(postgres.NewDB(testPool))
	entryRepo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	memberRepo := postgres.NewMemberRepository(postgres.NewDB(testPool))

	for i := 0; i < 2; i++ { // 10 ganados
		_ = entryRepo.Save(context.Background(), newEntry(t, ids.mk(), 5, now.Add(-time.Hour)))
	}
	item := saveItem(t, catRepo, 8)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			red, _ := domain.NewRedemption(domain.RedemptionID(randUUID()), memA, item, 8, "porque los dos queremos canjear ya", now)
			errs[idx] = redRepo.RedeemTx(context.Background(), red)
		}(i)
	}
	wg.Wait()

	failures := 0
	for _, err := range errs {
		if err == domain.ErrInsufficientBalance {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("exactamente un canje debía fallar por saldo, fallaron %d (errs=%v)", failures, errs)
	}

	bals, _ := memberRepo.Balances(context.Background())
	for _, b := range bals {
		if b.MemberID == memA && b.PointsAvailable < 0 {
			t.Fatalf("el saldo nunca debe quedar negativo, quedó en %d", b.PointsAvailable)
		}
	}
}

func TestCatalog_ListOnlyActive(t *testing.T) {
	truncate(t)
	repo := postgres.NewCatalogRepository(postgres.NewDB(testPool))
	active := saveItem(t, repo, 8)
	inactiveItem := saveItem(t, repo, 12)
	deact, _ := inactiveItem.Edit(inactiveItem.Name, "", inactiveItem.Cost, false, now)
	_ = repo.Update(context.Background(), deact)

	onlyActive, _ := repo.List(context.Background(), true)
	if len(onlyActive) != 1 || onlyActive[0].ID != active.ID {
		t.Fatalf("esperaba solo el activo, obtuve %+v", onlyActive)
	}
	all, _ := repo.List(context.Background(), false)
	if len(all) != 2 {
		t.Fatalf("esperaba 2 en el listado completo, obtuve %d", len(all))
	}
}
