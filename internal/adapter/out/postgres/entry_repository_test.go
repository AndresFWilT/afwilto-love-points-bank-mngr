//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/adapter/out/postgres"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

const (
	memA = domain.MemberID("11111111-1111-4111-8111-111111111111")
	memB = domain.MemberID("22222222-2222-4222-8222-222222222222")
)

var (
	ids = uuidGen{}
	now = time.Now().UTC().Truncate(time.Millisecond)
)

type uuidGen struct{}

func (uuidGen) mk() domain.EntryID { return domain.EntryID(randUUID()) }

func newEntry(t *testing.T, id domain.EntryID, amount int, occurredAt time.Time) domain.PointEntry {
	t.Helper()
	e, err := domain.NewPointEntry(id, memA, memB, amount, "porque hizo algo muy lindo hoy", occurredAt, now)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// Guardar y releer: la entidad vuelve idéntica, incluida la zona horaria.
func TestSaveAndFind(t *testing.T) {
	truncate(t)
	repo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	occurred := now.Add(-2 * time.Hour)
	e := newEntry(t, ids.mk(), 3, occurred)

	if err := repo.Save(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByID(context.Background(), e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != 3 || got.AwardedTo != memA || !got.OccurredAt.Equal(occurred) {
		t.Fatalf("entidad no coincide: %+v", got)
	}
	if got.Status != domain.StatusActive {
		t.Fatalf("esperaba activa, obtuve %v", got.Status)
	}
}

// CHECK de monto: un INSERT con amount fuera de rango es rechazado por la base.
// (El dominio no dejaría construir esto; lo forzamos con SQL directo para probar el CHECK.)
func TestCheckAmountRange(t *testing.T) {
	truncate(t)
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO point_entries (id, awarded_to, awarded_by, amount, reason, occurred_at)
		VALUES ($1,$2,$3,0,'razon suficientemente larga',now())`,
		randUUID(), memA.String(), memB.String())
	if err == nil {
		t.Fatal("la base debió rechazar amount 0")
	}
}

func TestCheckReasonLength(t *testing.T) {
	truncate(t)
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO point_entries (id, awarded_to, awarded_by, amount, reason, occurred_at)
		VALUES ($1,$2,$3,3,'corta',now())`,
		randUUID(), memA.String(), memB.String())
	if err == nil {
		t.Fatal("la base debió rechazar una razón de 5 caracteres")
	}
}

func TestCheckNoSelfAward(t *testing.T) {
	truncate(t)
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO point_entries (id, awarded_to, awarded_by, amount, reason, occurred_at)
		VALUES ($1,$2,$2,3,'razon suficientemente larga',now())`,
		randUUID(), memA.String())
	if err == nil {
		t.Fatal("la base debió rechazar awarded_by = awarded_to")
	}
}

// Edición transaccional: la entrada y la revisión se escriben juntas.
func TestUpdateWritesRevision(t *testing.T) {
	truncate(t)
	repo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	e := newEntry(t, ids.mk(), 3, now.Add(-time.Hour))
	_ = repo.Save(context.Background(), e)

	edited, rev, err := e.Edit(ids.mk(), 4, "porque además cocinó riquísimo", now.Add(-time.Hour), now, memB)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(context.Background(), edited, rev); err != nil {
		t.Fatal(err)
	}

	got, _ := repo.FindByID(context.Background(), e.ID)
	if got.Amount != 4 {
		t.Fatalf("esperaba 4, obtuve %d", got.Amount)
	}
	var count int
	_ = testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM point_entry_revisions WHERE entry_id=$1 AND prev_amount=3`, e.ID.String()).Scan(&count)
	if count != 1 {
		t.Fatalf("esperaba 1 revisión con prev_amount 3, obtuve %d", count)
	}
}

// Vista de saldos: las entradas anuladas no suman.
func TestBalanceViewIgnoresVoided(t *testing.T) {
	truncate(t)
	repo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	members := postgres.NewMemberRepository(postgres.NewDB(testPool))

	e1 := newEntry(t, ids.mk(), 3, now.Add(-time.Hour))
	e2 := newEntry(t, ids.mk(), 4, now.Add(-time.Hour))
	_ = repo.Save(context.Background(), e1)
	_ = repo.Save(context.Background(), e2)
	voided, _ := e2.Void(now, memB)
	_ = repo.UpdateStatus(context.Background(), voided)

	bals, err := members.Balances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bals {
		if b.MemberID == memA && b.PointsEarned != 3 {
			t.Fatalf("A debería tener 3 (la anulada no cuenta), obtuve %d", b.PointsEarned)
		}
	}
}

// Orden del libro y paginación por cursor: no salta ni repite en el borde.
func TestListOrderingAndCursor(t *testing.T) {
	truncate(t)
	repo := postgres.NewEntryRepository(postgres.NewDB(testPool))

	// tres entradas, dos con la misma fecha para forzar desempate por id.
	same := now.Add(-24 * time.Hour)
	older := now.Add(-48 * time.Hour)
	for _, spec := range []struct {
		amount     int
		occurredAt time.Time
	}{{1, same}, {2, same}, {3, older}} {
		e := newEntry(t, ids.mk(), spec.amount, spec.occurredAt)
		if err := repo.Save(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}

	first, err := repo.List(context.Background(), in.LedgerFilter{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 3 { // PageSize+1 = 3: la fila extra existe
		t.Fatalf("esperaba 3 filas (2+1), obtuve %d", len(first))
	}
	// orden: occurred_at DESC, id DESC
	for i := 1; i < len(first); i++ {
		if first[i-1].OccurredAt.Before(first[i].OccurredAt) {
			t.Fatalf("orden por fecha roto en %d", i)
		}
	}

	// segunda página desde el cursor de la 2ª fila
	cur := in.Cursor{OccurredAt: first[1].OccurredAt, ID: first[1].ID}
	second, err := repo.List(context.Background(), in.LedgerFilter{PageSize: 2, After: &cur})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[domain.EntryID]bool{first[0].ID: true, first[1].ID: true}
	for _, e := range second {
		if seen[e.ID] {
			t.Fatalf("el cursor repitió la fila %s", e.ID)
		}
	}
}

func TestFindByID_NotFound(t *testing.T) {
	truncate(t)
	repo := postgres.NewEntryRepository(postgres.NewDB(testPool))
	_, err := repo.FindByID(context.Background(), domain.EntryID(randUUID()))
	if !errors.Is(err, domain.ErrEntryNotFound) {
		t.Fatalf("esperaba ErrEntryNotFound, obtuve %v", err)
	}
}
