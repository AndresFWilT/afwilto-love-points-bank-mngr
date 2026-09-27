package testsupport

import (
	"context"
	"sync"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// MemoryRepo es un adaptador en memoria escrito a mano (nada de mocks generados).
// Implementa EntryRepository y MemberRepository para los tests de aplicación.
type MemoryRepo struct {
	mu          sync.Mutex
	members     []domain.Member
	entries     map[domain.EntryID]domain.PointEntry
	revisions   []domain.Revision
	items       map[domain.ItemID]domain.CatalogItem
	redemptions map[domain.RedemptionID]domain.Redemption
	// FailUpdate simula un fallo de persistencia para probar CA-5 (transaccionalidad).
	FailUpdate error
}

var (
	_ out.EntryRepository      = (*MemoryRepo)(nil)
	_ out.MemberRepository     = (*MemoryRepo)(nil)
	_ out.CatalogRepository    = (*MemoryCatalogRepo)(nil)
	_ out.RedemptionRepository = (*MemoryRedemptionRepo)(nil)
)

// CatalogRepo y RedemptionRepo son facetas sobre los mismos datos (FindByID choca de
// nombre entre las tres interfaces, así que no puede haber un solo tipo que las cumpla
// todas; en Postgres también son tipos separados).
func (r *MemoryRepo) CatalogRepo() *MemoryCatalogRepo         { return &MemoryCatalogRepo{base: r} }
func (r *MemoryRepo) RedemptionRepo() *MemoryRedemptionRepo   { return &MemoryRedemptionRepo{base: r} }

func NewMemoryRepo(members ...domain.Member) *MemoryRepo {
	return &MemoryRepo{
		members:     members,
		entries:     make(map[domain.EntryID]domain.PointEntry),
		items:       make(map[domain.ItemID]domain.CatalogItem),
		redemptions: make(map[domain.RedemptionID]domain.Redemption),
	}
}

func (r *MemoryRepo) Save(_ context.Context, e domain.PointEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.ID] = e
	return nil
}

func (r *MemoryRepo) Update(_ context.Context, e domain.PointEntry, rev domain.Revision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailUpdate != nil {
		// Ni la entrada ni la revisión se escriben: la transacción se revierte entera.
		return r.FailUpdate
	}
	r.entries[e.ID] = e
	r.revisions = append(r.revisions, rev)
	return nil
}

func (r *MemoryRepo) UpdateStatus(_ context.Context, e domain.PointEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.ID] = e
	return nil
}

func (r *MemoryRepo) FindByID(_ context.Context, id domain.EntryID) (domain.PointEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return domain.PointEntry{}, domain.ErrEntryNotFound
	}
	return e, nil
}

func (r *MemoryRepo) List(_ context.Context, f in.LedgerFilter) ([]domain.PointEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	all := make([]domain.PointEntry, 0, len(r.entries))
	for _, e := range r.entries {
		if f.AwardedTo != nil && e.AwardedTo != *f.AwardedTo {
			continue
		}
		if f.Status != nil && e.Status != *f.Status {
			continue
		}
		all = append(all, e)
	}
	domain.SortLedger(all)

	// Aplica el cursor: descarta todo lo que va antes o igual a la última fila vista.
	if f.After != nil {
		filtered := all[:0]
		for _, e := range all {
			if beforeCursor(e, *f.After) {
				filtered = append(filtered, e)
			}
		}
		all = filtered
	}

	// Devuelve hasta PageSize+1 para que el servicio calcule NextCursor.
	limit := f.PageSize + 1
	if f.PageSize <= 0 {
		limit = in.DefaultPageSize + 1
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// beforeCursor: e va estrictamente después del cursor en el orden del libro
// (occurred_at DESC, id DESC), es decir pertenece a una página posterior.
func beforeCursor(e domain.PointEntry, c in.Cursor) bool {
	if !e.OccurredAt.Equal(c.OccurredAt) {
		return e.OccurredAt.Before(c.OccurredAt)
	}
	return e.ID < c.ID
}

func (r *MemoryRepo) Exists(_ context.Context, id domain.MemberID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.members {
		if m.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryRepo) All(_ context.Context) ([]domain.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Member, len(r.members))
	copy(out, r.members)
	return out, nil
}

func (r *MemoryRepo) Balances(_ context.Context) ([]domain.Balance, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := make([]domain.PointEntry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}
	redemptions := make([]domain.Redemption, 0, len(r.redemptions))
	for _, rd := range r.redemptions {
		redemptions = append(redemptions, rd)
	}
	return domain.CalculateBalances(r.members, entries, redemptions), nil
}

// availableForLocked calcula el disponible de un titular. Debe llamarse con r.mu tomado.
func (r *MemoryRepo) availableForLocked(member domain.MemberID) int {
	earned := 0
	for _, e := range r.entries {
		if e.Status == domain.StatusActive && e.AwardedTo == member {
			earned += e.Amount
		}
	}
	redeemed := 0
	for _, rd := range r.redemptions {
		if rd.Status == domain.StatusActive && rd.MemberID == member {
			redeemed += rd.Cost
		}
	}
	return earned - redeemed
}

// Revisions expone las revisiones guardadas, para que los tests verifiquen RD-7.
func (r *MemoryRepo) Revisions() []domain.Revision {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Revision, len(r.revisions))
	copy(out, r.revisions)
	return out
}
