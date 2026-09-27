package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

var (
	memA  = domain.MemberID("11111111-1111-4111-8111-111111111111")
	memB  = domain.MemberID("22222222-2222-4222-8222-222222222222")
	id    = domain.EntryID("aaaa")
	revID = domain.EntryID("rev-1")
	now   = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	ayer  = now.Add(-24 * time.Hour)
)

// RD-1
func TestNewPointEntry_RejectsAmountOutOfRange(t *testing.T) {
	cases := []struct {
		name   string
		amount int
	}{
		{"cero", 0},
		{"negativo", -1},
		{"seis", 6},
		{"cien", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewPointEntry(id, memA, memB, tc.amount, "porque me contó del susto", ayer, now)
			if !errors.Is(err, domain.ErrAmountOutOfRange) {
				t.Fatalf("esperaba ErrAmountOutOfRange, obtuve %v", err)
			}
		})
	}
}

func TestNewPointEntry_AcceptsAmountBoundaries(t *testing.T) {
	for _, amt := range []int{1, 5} {
		if _, err := domain.NewPointEntry(id, memA, memB, amt, "porque me contó del susto", ayer, now); err != nil {
			t.Fatalf("amount=%d debería ser válido, obtuve %v", amt, err)
		}
	}
}

// RD-2
func TestNewPointEntry_RejectsShortReason(t *testing.T) {
	cases := []struct{ name, reason string }{
		{"vacía", ""},
		{"solo espacios", "          "},
		{"muy corta", "por linda"},        // 9 chars
		{"corta con espacios", "  hola  "}, // 4 útiles
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.NewPointEntry(id, memA, memB, 3, tc.reason, ayer, now)
			if !errors.Is(err, domain.ErrReasonTooShort) {
				t.Fatalf("esperaba ErrReasonTooShort, obtuve %v", err)
			}
		})
	}
}

func TestNewPointEntry_AcceptsExactlyTenChars(t *testing.T) {
	if _, err := domain.NewPointEntry(id, memA, memB, 3, "diez chars", ayer, now); err != nil {
		t.Fatalf("10 caracteres útiles deberían pasar, obtuve %v", err)
	}
}

func TestNewPointEntry_TrimsReason(t *testing.T) {
	e, err := domain.NewPointEntry(id, memA, memB, 3, "   porque me contó del susto   ", ayer, now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(e.Reason, " ") || strings.HasSuffix(e.Reason, " ") {
		t.Fatalf("reason debería quedar recortada, obtuve %q", e.Reason)
	}
}

// RD-3
func TestNewPointEntry_RejectsSelfAward(t *testing.T) {
	_, err := domain.NewPointEntry(id, memA, memA, 3, "porque me contó del susto", ayer, now)
	if !errors.Is(err, domain.ErrSelfAward) {
		t.Fatalf("esperaba ErrSelfAward, obtuve %v", err)
	}
}

// RD-4
func TestNewPointEntry_RejectsFutureDate(t *testing.T) {
	manana := now.Add(1 * time.Second)
	_, err := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", manana, now)
	if !errors.Is(err, domain.ErrFutureDate) {
		t.Fatalf("esperaba ErrFutureDate, obtuve %v", err)
	}
}

func TestNewPointEntry_AcceptsNow(t *testing.T) {
	if _, err := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", now, now); err != nil {
		t.Fatalf("occurred_at == now debería ser válido, obtuve %v", err)
	}
}

func TestNewPointEntry_HappyPath(t *testing.T) {
	e, err := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)
	if err != nil {
		t.Fatal(err)
	}
	if e.Status != domain.StatusActive {
		t.Fatalf("una entrada nace activa, obtuve %v", e.Status)
	}
	if !e.CreatedAt.Equal(now) || !e.UpdatedAt.Equal(now) {
		t.Fatalf("created/updated deberían ser now")
	}
}

// RD-5, RD-7
func TestEdit_ProducesRevisionWithPrevValues(t *testing.T) {
	e, _ := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)
	later := now.Add(time.Hour)

	edited, rev, err := e.Edit(revID, 4, "porque además cocinó rico", ayer, later, memB)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Amount != 4 {
		t.Fatalf("esperaba amount 4, obtuve %d", edited.Amount)
	}
	if !edited.UpdatedAt.Equal(later) {
		t.Fatalf("updated_at debería avanzar")
	}
	if rev.PrevAmount != 3 || rev.PrevReason != "porque me contó del susto" {
		t.Fatalf("la revisión debe guardar valores anteriores, obtuve %+v", rev)
	}
	if rev.EditedBy != memB || rev.EntryID != e.ID {
		t.Fatalf("revisión mal atribuida: %+v", rev)
	}
}

// RD-5
func TestEdit_RejectsNonAwarder(t *testing.T) {
	e, _ := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)
	_, _, err := e.Edit(revID, 4, "porque además cocinó rico", ayer, now, memA)
	if !errors.Is(err, domain.ErrNotTheAwarder) {
		t.Fatalf("esperaba ErrNotTheAwarder, obtuve %v", err)
	}
}

// RD-6
func TestEdit_RejectsVoidedEntry(t *testing.T) {
	e, _ := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)
	voided, _ := e.Void(now, memB)
	_, _, err := voided.Edit(revID, 4, "porque además cocinó rico", ayer, now, memB)
	if !errors.Is(err, domain.ErrEntryVoided) {
		t.Fatalf("esperaba ErrEntryVoided, obtuve %v", err)
	}
}

// RD-1..RD-4 revalidadas en edición
func TestEdit_RevalidatesInvariants(t *testing.T) {
	e, _ := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)
	if _, _, err := e.Edit(revID, 9, "porque además cocinó rico", ayer, now, memB); !errors.Is(err, domain.ErrAmountOutOfRange) {
		t.Fatalf("edición con amount 9 debe fallar, obtuve %v", err)
	}
	if _, _, err := e.Edit(revID, 4, "corto", ayer, now, memB); !errors.Is(err, domain.ErrReasonTooShort) {
		t.Fatalf("edición con razón corta debe fallar, obtuve %v", err)
	}
}

// RD-5, RD-6, RD-Void-1
func TestVoid(t *testing.T) {
	e, _ := domain.NewPointEntry(id, memA, memB, 3, "porque me contó del susto", ayer, now)

	t.Run("no autor", func(t *testing.T) {
		if _, err := e.Void(now, memA); !errors.Is(err, domain.ErrNotTheAwarder) {
			t.Fatalf("esperaba ErrNotTheAwarder, obtuve %v", err)
		}
	})
	t.Run("autor anula", func(t *testing.T) {
		v, err := e.Void(now, memB)
		if err != nil {
			t.Fatal(err)
		}
		if v.Status != domain.StatusVoided {
			t.Fatalf("esperaba voided, obtuve %v", v.Status)
		}
	})
	t.Run("doble anulación", func(t *testing.T) {
		v, _ := e.Void(now, memB)
		if _, err := v.Void(now, memB); !errors.Is(err, domain.ErrEntryVoided) {
			t.Fatalf("esperaba ErrEntryVoided, obtuve %v", err)
		}
	})
}
