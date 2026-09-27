package testsupport

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// MemoryCatalogRepo es una faceta de MemoryRepo sobre el catálogo (escrita a mano).
type MemoryCatalogRepo struct{ base *MemoryRepo }

func (r *MemoryCatalogRepo) Save(_ context.Context, item domain.CatalogItem) error {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	r.base.items[item.ID] = item
	return nil
}

func (r *MemoryCatalogRepo) Update(_ context.Context, item domain.CatalogItem) error {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	r.base.items[item.ID] = item
	return nil
}

func (r *MemoryCatalogRepo) FindByID(_ context.Context, id domain.ItemID) (domain.CatalogItem, error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	item, ok := r.base.items[id]
	if !ok {
		return domain.CatalogItem{}, domain.ErrItemNotFound
	}
	return item, nil
}

func (r *MemoryCatalogRepo) List(_ context.Context, onlyActive bool) ([]domain.CatalogItem, error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	out := make([]domain.CatalogItem, 0, len(r.base.items))
	for _, item := range r.base.items {
		if onlyActive && !item.IsActive {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// MemoryRedemptionRepo es una faceta de MemoryRepo sobre los canjes.
type MemoryRedemptionRepo struct{ base *MemoryRepo }

// RedeemTx verifica el saldo disponible dentro del "lock" (simula la transacción) y solo
// inserta si alcanza (RD-CAN-2).
func (r *MemoryRedemptionRepo) RedeemTx(_ context.Context, red domain.Redemption) error {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	if red.Cost > r.base.availableForLocked(red.MemberID) {
		return domain.ErrInsufficientBalance
	}
	r.base.redemptions[red.ID] = red
	return nil
}

func (r *MemoryRedemptionRepo) UpdateStatus(_ context.Context, red domain.Redemption) error {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	r.base.redemptions[red.ID] = red
	return nil
}

func (r *MemoryRedemptionRepo) FindByID(_ context.Context, id domain.RedemptionID) (domain.Redemption, error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	red, ok := r.base.redemptions[id]
	if !ok {
		return domain.Redemption{}, domain.ErrRedemptionNotFound
	}
	return red, nil
}

func (r *MemoryRedemptionRepo) List(_ context.Context, f in.RedemptionFilter) ([]domain.Redemption, error) {
	r.base.mu.Lock()
	defer r.base.mu.Unlock()
	out := make([]domain.Redemption, 0, len(r.base.redemptions))
	for _, red := range r.base.redemptions {
		if f.MemberID != nil && red.MemberID != *f.MemberID {
			continue
		}
		out = append(out, red)
	}
	return out, nil
}
