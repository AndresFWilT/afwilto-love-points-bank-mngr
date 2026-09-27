package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/service"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/testsupport"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

var (
	memA = domain.MemberID("11111111-1111-4111-8111-111111111111")
	memB = domain.MemberID("22222222-2222-4222-8222-222222222222")
	memX = domain.MemberID("99999999-9999-4999-8999-999999999999")
	now  = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	ayer = now.Add(-24 * time.Hour)
)

func newService(repo *testsupport.MemoryRepo) *service.PointsService {
	return service.New(repo, repo, testsupport.FixedClock{T: now}, &testsupport.SeqIDs{})
}

func seededRepo() *testsupport.MemoryRepo {
	return testsupport.NewMemoryRepo(
		domain.Member{ID: memA, DisplayName: "Titular A"},
		domain.Member{ID: memB, DisplayName: "Titular B"},
	)
}

// spec 001 CA-1
func TestAward_HappyPath(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)

	e, err := svc.Award(context.Background(), in.AwardCommand{
		AwardedTo: memA, AwardedBy: memB, Amount: 3,
		Reason: "porque me contó del susto", OccurredAt: ayer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != domain.StatusActive || e.ID == "" {
		t.Fatalf("entrada inválida: %+v", e)
	}
}

// spec 001 CA-6
func TestAward_MemberNotFound(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)

	_, err := svc.Award(context.Background(), in.AwardCommand{
		AwardedTo: memX, AwardedBy: memB, Amount: 3,
		Reason: "porque me contó del susto", OccurredAt: ayer,
	})
	if !errors.Is(err, domain.ErrMemberNotFound) {
		t.Fatalf("esperaba ErrMemberNotFound, obtuve %v", err)
	}
}

// spec 001 CA-2..CA-5: el servicio propaga los errores del dominio
func TestAward_PropagatesDomainErrors(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	base := in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 3, Reason: "porque me contó del susto", OccurredAt: ayer}

	cases := []struct {
		name string
		mut  func(c in.AwardCommand) in.AwardCommand
		want error
	}{
		{"amount", func(c in.AwardCommand) in.AwardCommand { c.Amount = 0; return c }, domain.ErrAmountOutOfRange},
		{"reason", func(c in.AwardCommand) in.AwardCommand { c.Reason = "corto"; return c }, domain.ErrReasonTooShort},
		{"self", func(c in.AwardCommand) in.AwardCommand { c.AwardedTo = memB; return c }, domain.ErrSelfAward},
		{"future", func(c in.AwardCommand) in.AwardCommand { c.OccurredAt = now.Add(time.Hour); return c }, domain.ErrFutureDate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Award(context.Background(), tc.mut(base))
			if !errors.Is(err, tc.want) {
				t.Fatalf("esperaba %v, obtuve %v", tc.want, err)
			}
		})
	}
}

// spec 003 CA-1, RD-7
func TestEdit_WritesRevision(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	e, _ := svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 3, Reason: "porque me contó del susto", OccurredAt: ayer})

	edited, err := svc.Edit(context.Background(), in.EditCommand{
		EntryID: e.ID, EditedBy: memB, Amount: 4, Reason: "porque además cocinó rico", OccurredAt: ayer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Amount != 4 {
		t.Fatalf("esperaba amount 4, obtuve %d", edited.Amount)
	}
	revs := repo.Revisions()
	if len(revs) != 1 || revs[0].PrevAmount != 3 {
		t.Fatalf("esperaba una revisión con prev_amount 3, obtuve %+v", revs)
	}
}

// spec 003 CA-2
func TestEdit_NotTheAwarder(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	e, _ := svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 3, Reason: "porque me contó del susto", OccurredAt: ayer})

	_, err := svc.Edit(context.Background(), in.EditCommand{EntryID: e.ID, EditedBy: memA, Amount: 4, Reason: "porque además cocinó rico", OccurredAt: ayer})
	if !errors.Is(err, domain.ErrNotTheAwarder) {
		t.Fatalf("esperaba ErrNotTheAwarder, obtuve %v", err)
	}
}

// spec 003 CA-5: si falla la persistencia, nada se escribe
func TestEdit_TransactionalFailureLeavesNoRevision(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	e, _ := svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 3, Reason: "porque me contó del susto", OccurredAt: ayer})

	repo.FailUpdate = errors.New("boom")
	_, err := svc.Edit(context.Background(), in.EditCommand{EntryID: e.ID, EditedBy: memB, Amount: 4, Reason: "porque además cocinó rico", OccurredAt: ayer})
	if err == nil {
		t.Fatal("esperaba error de persistencia")
	}
	if len(repo.Revisions()) != 0 {
		t.Fatalf("no debería haber revisiones tras el fallo, hay %d", len(repo.Revisions()))
	}
}

// spec 004 CA-1, CA-4
func TestVoid_RemovesFromBalance(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	e, _ := svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 4, Reason: "porque me contó del susto", OccurredAt: ayer})

	if _, err := svc.Void(context.Background(), e.ID, memB); err != nil {
		t.Fatal(err)
	}
	bals, _ := svc.Balances(context.Background())
	for _, b := range bals {
		if b.MemberID == memA && b.PointsEarned != 0 {
			t.Fatalf("tras anular, A debería tener 0, obtuve %d", b.PointsEarned)
		}
	}
}

// spec 004 CA-3
func TestVoid_DoubleVoidConflicts(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	e, _ := svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 4, Reason: "porque me contó del susto", OccurredAt: ayer})
	_, _ = svc.Void(context.Background(), e.ID, memB)

	if _, err := svc.Void(context.Background(), e.ID, memB); !errors.Is(err, domain.ErrEntryVoided) {
		t.Fatalf("esperaba ErrEntryVoided, obtuve %v", err)
	}
}

// spec 002 CA-1: orden del libro
func TestLedger_Ordering(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	mk := func(d time.Duration, reason string) {
		_, _ = svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 1, Reason: reason, OccurredAt: now.Add(d)})
	}
	mk(-72*time.Hour, "porque llegó temprano hoy")
	mk(-1*time.Hour, "porque me trajo café hoy")
	mk(-48*time.Hour, "porque me escribió lindo")

	page, err := svc.Ledger(context.Background(), in.LedgerFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 3 {
		t.Fatalf("esperaba 3 entradas, obtuve %d", len(page.Entries))
	}
	for i := 1; i < len(page.Entries); i++ {
		if page.Entries[i-1].OccurredAt.Before(page.Entries[i].OccurredAt) {
			t.Fatalf("libro desordenado en la posición %d", i)
		}
	}
}

// spec 002 CA-2: libro vacío
func TestLedger_Empty(t *testing.T) {
	svc := newService(seededRepo())
	page, err := svc.Ledger(context.Background(), in.LedgerFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 || page.NextCursor != nil {
		t.Fatalf("esperaba vacío y sin cursor, obtuve %+v", page)
	}
}

// spec 002 CA-3: paginación por cursor sin saltos ni repeticiones
func TestLedger_CursorPagination(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	for i := 0; i < 3; i++ {
		_, _ = svc.Award(context.Background(), in.AwardCommand{
			AwardedTo: memA, AwardedBy: memB, Amount: 1,
			Reason: "porque hizo algo lindo hoy", OccurredAt: now.Add(time.Duration(-i) * time.Hour),
		})
	}

	first, _ := svc.Ledger(context.Background(), in.LedgerFilter{PageSize: 2})
	if len(first.Entries) != 2 || first.NextCursor == nil {
		t.Fatalf("primera página debería traer 2 y un cursor, obtuve %d cursor=%v", len(first.Entries), first.NextCursor)
	}

	second, _ := svc.Ledger(context.Background(), in.LedgerFilter{PageSize: 2, After: first.NextCursor})
	if len(second.Entries) != 1 {
		t.Fatalf("segunda página debería traer 1, obtuve %d", len(second.Entries))
	}
	// sin repetición: ningún id de la segunda aparece en la primera
	seen := map[domain.EntryID]bool{}
	for _, e := range first.Entries {
		seen[e.ID] = true
	}
	for _, e := range second.Entries {
		if seen[e.ID] {
			t.Fatalf("el cursor repitió la fila %s", e.ID)
		}
	}
	if second.NextCursor != nil {
		t.Fatalf("no debería haber tercera página")
	}
}

// spec 002 CA-4: filtro por destinatario
func TestLedger_FilterByAwardedTo(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	_, _ = svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 1, Reason: "porque hizo algo lindo hoy", OccurredAt: ayer})
	_, _ = svc.Award(context.Background(), in.AwardCommand{AwardedTo: memB, AwardedBy: memA, Amount: 1, Reason: "porque hizo algo lindo hoy", OccurredAt: ayer})

	page, _ := svc.Ledger(context.Background(), in.LedgerFilter{AwardedTo: &memA})
	if len(page.Entries) != 1 || page.Entries[0].AwardedTo != memA {
		t.Fatalf("el filtro por awarded_to falló: %+v", page.Entries)
	}
}

// spec 005 CA-1, CA-3
func TestBalances(t *testing.T) {
	repo := seededRepo()
	svc := newService(repo)
	_, _ = svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 3, Reason: "porque me contó del susto", OccurredAt: ayer})
	_, _ = svc.Award(context.Background(), in.AwardCommand{AwardedTo: memA, AwardedBy: memB, Amount: 2, Reason: "porque llegó con arepas", OccurredAt: ayer})

	bals, _ := svc.Balances(context.Background())
	m := map[domain.MemberID]domain.Balance{}
	for _, b := range bals {
		m[b.MemberID] = b
	}
	if m[memA].PointsEarned != 5 {
		t.Fatalf("A debería tener 5, obtuve %d", m[memA].PointsEarned)
	}
	if _, ok := m[memB]; !ok || m[memB].PointsEarned != 0 {
		t.Fatalf("B debería aparecer con 0")
	}
}
