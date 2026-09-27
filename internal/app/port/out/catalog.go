package out

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type CatalogRepository interface {
	Save(ctx context.Context, item domain.CatalogItem) error
	Update(ctx context.Context, item domain.CatalogItem) error
	FindByID(ctx context.Context, id domain.ItemID) (domain.CatalogItem, error)
	List(ctx context.Context, onlyActive bool) ([]domain.CatalogItem, error)
}

type RedemptionRepository interface {
	// RedeemTx corre en una transacción: bloquea al titular, recalcula el disponible y
	// solo si amount ≤ disponible inserta el canje. Devuelve ErrInsufficientBalance si no
	// alcanza. Aquí vive la garantía anti-sobregiro (RD-CAN-2) a prueba de concurrencia.
	RedeemTx(ctx context.Context, r domain.Redemption) error
	UpdateStatus(ctx context.Context, r domain.Redemption) error
	FindByID(ctx context.Context, id domain.RedemptionID) (domain.Redemption, error)
	List(ctx context.Context, f in.RedemptionFilter) ([]domain.Redemption, error)
}
