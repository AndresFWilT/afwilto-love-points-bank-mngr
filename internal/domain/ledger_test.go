package domain_test

import (
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

func members() []domain.Member {
	return []domain.Member{
		{ID: memA, DisplayName: "Titular A"},
		{ID: memB, DisplayName: "Titular B"},
	}
}

// RD-8
func TestCalculateBalances_SumsActiveEntries(t *testing.T) {
	e1, _ := domain.NewPointEntry("1", memA, memB, 3, "porque me contó del susto", ayer, now)
	e2, _ := domain.NewPointEntry("2", memA, memB, 2, "porque llegó con arepas", ayer, now)
	bals := domain.CalculateBalances(members(), []domain.PointEntry{e1, e2}, nil)

	got := byID(bals)
	if got[memA].PointsEarned != 5 {
		t.Fatalf("A debería tener 5, obtuve %d", got[memA].PointsEarned)
	}
	if got[memA].EntriesCount != 2 {
		t.Fatalf("A debería tener 2 entradas, obtuve %d", got[memA].EntriesCount)
	}
}

// RD-8: las anuladas no cuentan
func TestCalculateBalances_IgnoresVoided(t *testing.T) {
	e1, _ := domain.NewPointEntry("1", memA, memB, 3, "porque me contó del susto", ayer, now)
	e2, _ := domain.NewPointEntry("2", memA, memB, 4, "porque llegó con arepas", ayer, now)
	e2, _ = e2.Void(now, memB)

	bals := byID(domain.CalculateBalances(members(), []domain.PointEntry{e1, e2}, nil))
	if bals[memA].PointsEarned != 3 {
		t.Fatalf("las anuladas no suman: esperaba 3, obtuve %d", bals[memA].PointsEarned)
	}
}

// RD-Bal-1: titular sin entradas aparece con 0
func TestCalculateBalances_ZeroForMemberWithoutEntries(t *testing.T) {
	e1, _ := domain.NewPointEntry("1", memA, memB, 3, "porque me contó del susto", ayer, now)
	bals := byID(domain.CalculateBalances(members(), []domain.PointEntry{e1}, nil))
	b, ok := bals[memB]
	if !ok {
		t.Fatal("B debería aparecer aunque no tenga entradas")
	}
	if b.PointsEarned != 0 || b.LastEntryAt != nil {
		t.Fatalf("B debería tener 0 y last_entry_at nil, obtuve %+v", b)
	}
}

func TestCalculateBalances_LastEntryAtIsMostRecent(t *testing.T) {
	old := now.Add(-72 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	e1, _ := domain.NewPointEntry("1", memA, memB, 3, "porque me contó del susto", old, now)
	e2, _ := domain.NewPointEntry("2", memA, memB, 2, "porque llegó con arepas", recent, now)
	bals := byID(domain.CalculateBalances(members(), []domain.PointEntry{e1, e2}, nil))
	if !bals[memA].LastEntryAt.Equal(recent) {
		t.Fatalf("last_entry_at debería ser la más reciente")
	}
}

// RD-8b/8c (fase 2): los canjes activos bajan el disponible; los anulados no.
func TestCalculateBalances_SubtractsActiveRedemptions(t *testing.T) {
	e1, _ := domain.NewPointEntry("1", memA, memB, 5, "porque me contó del susto", ayer, now)
	e2, _ := domain.NewPointEntry("2", memA, memB, 5, "porque llegó con arepas", ayer, now)
	item, _ := domain.NewCatalogItem("plan-1", "Noche de película", "", 8, now)
	r1, _ := domain.NewRedemption("r1", memA, item, 3, "porque nos lo merecemos hoy", now)
	r2, _ := domain.NewRedemption("r2", memA, item, 4, "porque terminamos el proyecto", now)
	r2, _ = r2.Void(now, memA) // este no cuenta

	bals := byID(domain.CalculateBalances(members(),
		[]domain.PointEntry{e1, e2}, []domain.Redemption{r1, r2}))

	a := bals[memA]
	if a.PointsEarned != 10 || a.PointsRedeemed != 3 || a.PointsAvailable != 7 {
		t.Fatalf("esperaba ganado=10 canjeado=3 disponible=7, obtuve %+v", a)
	}
}

func TestSortLedger_ByOccurredThenID(t *testing.T) {
	t0 := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	// dos con misma fecha para probar desempate por id DESC
	a := domain.RehydratePointEntry("a", memA, memB, 1, "x", t1, domain.StatusActive, now, now)
	b := domain.RehydratePointEntry("b", memA, memB, 1, "x", t1, domain.StatusActive, now, now)
	c := domain.RehydratePointEntry("c", memA, memB, 1, "x", t0, domain.StatusActive, now, now)

	entries := []domain.PointEntry{c, a, b}
	domain.SortLedger(entries)

	want := []domain.EntryID{"b", "a", "c"} // t1 primero, id DESC; t0 al final
	for i, w := range want {
		if entries[i].ID != w {
			t.Fatalf("posición %d: esperaba %s, obtuve %s", i, w, entries[i].ID)
		}
	}
}

func byID(bals []domain.Balance) map[domain.MemberID]domain.Balance {
	m := make(map[domain.MemberID]domain.Balance, len(bals))
	for _, b := range bals {
		m[b.MemberID] = b
	}
	return m
}
