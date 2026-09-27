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

type fakeCatalog struct {
	create func(in.CreateItemCommand) (domain.CatalogItem, error)
	edit   func(in.EditItemCommand) (domain.CatalogItem, error)
	list   func(bool) ([]domain.CatalogItem, error)
}

func (f fakeCatalog) CreateItem(_ context.Context, c in.CreateItemCommand) (domain.CatalogItem, error) {
	return f.create(c)
}
func (f fakeCatalog) EditItem(_ context.Context, c in.EditItemCommand) (domain.CatalogItem, error) {
	return f.edit(c)
}
func (f fakeCatalog) ListItems(_ context.Context, active bool) ([]domain.CatalogItem, error) {
	return f.list(active)
}

type fakeRedemption struct {
	redeem func(in.RedeemCommand) (domain.Redemption, error)
	void   func(domain.RedemptionID, domain.MemberID) (domain.Redemption, error)
	list   func(in.RedemptionFilter) ([]domain.Redemption, error)
}

func (f fakeRedemption) Redeem(_ context.Context, c in.RedeemCommand) (domain.Redemption, error) {
	return f.redeem(c)
}
func (f fakeRedemption) Void(_ context.Context, id domain.RedemptionID, by domain.MemberID) (domain.Redemption, error) {
	return f.void(id, by)
}
func (f fakeRedemption) List(_ context.Context, fl in.RedemptionFilter) ([]domain.Redemption, error) {
	return f.list(fl)
}

func doV2(t *testing.T, uc adapter.UseCases, method, path, member, body string) *httptest.ResponseRecorder {
	t.Helper()
	srv := adapter.NewRouter(uc, "")
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

// spec 006 CA-1
func TestCreateCatalogItem_201WithDerivedTier(t *testing.T) {
	item, _ := domain.NewCatalogItem("i1", "Noche de película", "", 8, time.Now())
	var got in.CreateItemCommand
	uc := adapter.UseCases{Catalog: fakeCatalog{create: func(c in.CreateItemCommand) (domain.CatalogItem, error) {
		got = c
		return item, nil
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/catalog-items", memB, `{"name":"Noche de película","cost":8}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", w.Code, w.Body.String())
	}
	if got.Cost != 8 {
		t.Fatalf("cost debería propagarse, obtuve %d", got.Cost)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["tier"] != "medio" {
		t.Fatalf("tier derivado debería ser medio, obtuve %v", out["tier"])
	}
}

// spec 006: falta identidad
func TestCreateCatalogItem_401(t *testing.T) {
	uc := adapter.UseCases{Catalog: fakeCatalog{create: func(in.CreateItemCommand) (domain.CatalogItem, error) {
		t.Fatal("no debió llamarse")
		return domain.CatalogItem{}, nil
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/catalog-items", "", `{"name":"x","cost":8}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("esperaba 401, obtuve %d", w.Code)
	}
}

// spec 006 CA-2: cost inválido → 422
func TestCreateCatalogItem_422(t *testing.T) {
	uc := adapter.UseCases{Catalog: fakeCatalog{create: func(in.CreateItemCommand) (domain.CatalogItem, error) {
		return domain.CatalogItem{}, domain.ErrCostNotPositive
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/catalog-items", memB, `{"name":"Noche de película","cost":0}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("esperaba 422, obtuve %d", w.Code)
	}
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p["code"] != "cost_not_positive" {
		t.Fatalf("esperaba code cost_not_positive, obtuve %v", p["code"])
	}
}

// spec 007 CA-1
func TestRedeem_201(t *testing.T) {
	red := domain.RehydrateRedemption("r1", memB, "i1", 8, "porque nos lo merecemos hoy", domain.StatusActive, time.Now(), time.Now())
	var got in.RedeemCommand
	uc := adapter.UseCases{Redemptions: fakeRedemption{redeem: func(c in.RedeemCommand) (domain.Redemption, error) {
		got = c
		return red, nil
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/redemptions", memB,
		`{"catalog_item_id":"i1","amount":8,"note":"porque nos lo merecemos hoy"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("esperaba 201, obtuve %d (%s)", w.Code, w.Body.String())
	}
	if got.RedeemedBy != domain.MemberID(memB) || got.Amount != 8 {
		t.Fatalf("comando inesperado: %+v", got)
	}
}

// spec 007 CA-2: saldo insuficiente → 422 insufficient_balance
func TestRedeem_InsufficientBalance_422(t *testing.T) {
	uc := adapter.UseCases{Redemptions: fakeRedemption{redeem: func(in.RedeemCommand) (domain.Redemption, error) {
		return domain.Redemption{}, domain.ErrInsufficientBalance
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/redemptions", memB,
		`{"catalog_item_id":"i1","amount":99,"note":"porque quiero gastar de más hoy"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("esperaba 422, obtuve %d", w.Code)
	}
	var p map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p["code"] != "insufficient_balance" {
		t.Fatalf("esperaba insufficient_balance, obtuve %v", p["code"])
	}
}

// spec 007 CA-4: plan inactivo → 409
func TestRedeem_InactiveItem_409(t *testing.T) {
	uc := adapter.UseCases{Redemptions: fakeRedemption{redeem: func(in.RedeemCommand) (domain.Redemption, error) {
		return domain.Redemption{}, domain.ErrItemInactive
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/redemptions", memB,
		`{"catalog_item_id":"i1","amount":8,"note":"porque nos lo merecemos hoy"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("esperaba 409, obtuve %d", w.Code)
	}
}

// spec 007 CA-7: anular sin ser el que canjeó → 403
func TestVoidRedemption_403(t *testing.T) {
	uc := adapter.UseCases{Redemptions: fakeRedemption{void: func(domain.RedemptionID, domain.MemberID) (domain.Redemption, error) {
		return domain.Redemption{}, domain.ErrNotTheRedeemer
	}}}
	w := doV2(t, uc, http.MethodPost, "/api/v1/redemptions/r1/void", memB, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("esperaba 403, obtuve %d", w.Code)
	}
}
