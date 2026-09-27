package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	adapter "github.com/andresfwilt/banco-de-puntitos/internal/adapter/in/http"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// fakeUC es un doble a mano del caso de uso: registra lo recibido y devuelve lo pactado.
type fakeUC struct {
	award    func(in.AwardCommand) (domain.PointEntry, error)
	edit     func(in.EditCommand) (domain.PointEntry, error)
	void     func(domain.EntryID, domain.MemberID) (domain.PointEntry, error)
	ledger   func(in.LedgerFilter) (in.LedgerPage, error)
	balances func() ([]domain.Balance, error)
	members  func() ([]domain.Member, error)
}

func (f fakeUC) Award(_ context.Context, c in.AwardCommand) (domain.PointEntry, error) {
	return f.award(c)
}
func (f fakeUC) Edit(_ context.Context, c in.EditCommand) (domain.PointEntry, error) {
	return f.edit(c)
}
func (f fakeUC) Void(_ context.Context, id domain.EntryID, by domain.MemberID) (domain.PointEntry, error) {
	return f.void(id, by)
}
func (f fakeUC) Ledger(_ context.Context, fl in.LedgerFilter) (in.LedgerPage, error) {
	return f.ledger(fl)
}
func (f fakeUC) Balances(_ context.Context) ([]domain.Balance, error) { return f.balances() }
func (f fakeUC) Members(_ context.Context) ([]domain.Member, error)   { return f.members() }

const memB = "22222222-2222-4222-8222-222222222222"

func do(t *testing.T, uc in.PointsUseCase, method, path, member, body string) *httptest.ResponseRecorder {
	t.Helper()
	srv := adapter.NewRouter(adapter.UseCases{Points: uc}, "")
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if member != "" {
		r.Header.Set("X-Member-Id", member)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, r)
	return w
}

// spec 000 CA-1
func TestAward_MissingMemberHeader_401(t *testing.T) {
	uc := fakeUC{award: func(in.AwardCommand) (domain.PointEntry, error) { t.Fatal("no debió llamarse"); return domain.PointEntry{}, nil }}
	w := do(t, uc, http.MethodPost, "/api/v1/entries", "", `{"awarded_to":"x","amount":3,"reason":"porque me contó del susto","occurred_at":"2026-09-10T00:00:00Z"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401, obtuve %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("esperaba problem+json, obtuve %q", ct)
	}
}

// spec 001 CA-1
func TestAward_Created_201(t *testing.T) {
	entry := domain.RehydratePointEntry("e1", "11111111-1111-4111-8111-111111111111", memB, 3, "porque me contó del susto", time.Now().Add(-time.Hour), domain.StatusActive, time.Now(), time.Now())
	var got in.AwardCommand
	uc := fakeUC{award: func(c in.AwardCommand) (domain.PointEntry, error) { got = c; return entry, nil }}

	w := do(t, uc, http.MethodPost, "/api/v1/entries", memB, `{"awarded_to":"11111111-1111-4111-8111-111111111111","amount":3,"reason":"porque me contó del susto","occurred_at":"2026-09-10T00:00:00Z"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", w.Code, w.Body.String())
	}
	if got.AwardedBy != domain.MemberID(memB) {
		t.Fatalf("awarded_by debería venir del header, obtuve %q", got.AwardedBy)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["id"] != "e1" {
		t.Fatalf("respuesta inesperada: %s", w.Body.String())
	}
}

// spec 001 CA-3: error de dominio → 422 problem+json con code
func TestAward_ReasonTooShort_422(t *testing.T) {
	uc := fakeUC{award: func(in.AwardCommand) (domain.PointEntry, error) { return domain.PointEntry{}, domain.ErrReasonTooShort }}
	w := do(t, uc, http.MethodPost, "/api/v1/entries", memB, `{"awarded_to":"x","amount":3,"reason":"corto","occurred_at":"2026-09-10T00:00:00Z"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("esperaba 422, obtuve %d", w.Code)
	}
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p["code"] != "reason_too_short" {
		t.Fatalf("esperaba code reason_too_short, obtuve %v", p["code"])
	}
}

// spec 003 CA-2: forbidden → 403
func TestEdit_NotAwarder_403(t *testing.T) {
	uc := fakeUC{edit: func(in.EditCommand) (domain.PointEntry, error) { return domain.PointEntry{}, domain.ErrNotTheAwarder }}
	w := do(t, uc, http.MethodPatch, "/api/v1/entries/e1", memB, `{"amount":4,"reason":"porque además cocinó rico","occurred_at":"2026-09-10T00:00:00Z"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("esperaba 403, obtuve %d", w.Code)
	}
}

// spec 004 CA-3: voided → 409
func TestVoid_AlreadyVoided_409(t *testing.T) {
	uc := fakeUC{void: func(domain.EntryID, domain.MemberID) (domain.PointEntry, error) { return domain.PointEntry{}, domain.ErrEntryVoided }}
	w := do(t, uc, http.MethodPost, "/api/v1/entries/e1/void", memB, "")
	if w.Code != http.StatusConflict {
		t.Fatalf("esperaba 409, obtuve %d", w.Code)
	}
}

// spec 002: listado y cursor opaco de ida y vuelta
func TestList_ReturnsCursor(t *testing.T) {
	occurred := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	entry := domain.RehydratePointEntry("e1", "a", memB, 2, "porque hizo algo lindo hoy", occurred, domain.StatusActive, occurred, occurred)
	var gotFilter in.LedgerFilter
	uc := fakeUC{ledger: func(f in.LedgerFilter) (in.LedgerPage, error) {
		gotFilter = f
		return in.LedgerPage{Entries: []domain.PointEntry{entry}, NextCursor: &in.Cursor{OccurredAt: occurred, ID: "e1"}}, nil
	}}

	w := do(t, uc, http.MethodGet, "/api/v1/entries?page_size=1", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", w.Code)
	}
	if gotFilter.PageSize != 1 {
		t.Fatalf("page_size debería propagarse, obtuve %d", gotFilter.PageSize)
	}
	var page struct {
		Entries    []map[string]any `json:"entries"`
		NextCursor *string          `json:"next_cursor"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if page.NextCursor == nil || *page.NextCursor == "" {
		t.Fatalf("esperaba un cursor no vacío")
	}

	// El cursor devuelto, reenviado, se decodifica al mismo instante+id.
	uc2 := fakeUC{ledger: func(f in.LedgerFilter) (in.LedgerPage, error) {
		if f.After == nil || f.After.ID != "e1" || !f.After.OccurredAt.Equal(occurred) {
			t.Fatalf("el cursor no se decodificó bien: %+v", f.After)
		}
		return in.LedgerPage{}, nil
	}}
	do(t, uc2, http.MethodGet, "/api/v1/entries?cursor="+*page.NextCursor, "", "")
}

// spec 005
func TestBalances_OK(t *testing.T) {
	uc := fakeUC{balances: func() ([]domain.Balance, error) {
		return []domain.Balance{{MemberID: "a", DisplayName: "Titular A", PointsEarned: 5}}, nil
	}}
	w := do(t, uc, http.MethodGet, "/api/v1/balances", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", w.Code)
	}
	var bals []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &bals)
	if len(bals) != 1 || bals[0]["points_earned"].(float64) != 5 {
		t.Fatalf("saldos inesperados: %s", w.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	uc := fakeUC{}
	w := do(t, uc, http.MethodGet, "/healthz", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("esperaba 200, obtuve %d", w.Code)
	}
}
