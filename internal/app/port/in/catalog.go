package in

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// --- Catálogo (spec 006) ---

type CreateItemCommand struct {
	Name        string
	Description string
	Cost        int
}

type EditItemCommand struct {
	ItemID      domain.ItemID
	Name        string
	Description string
	Cost        int
	IsActive    bool
}

type CatalogUseCase interface {
	CreateItem(ctx context.Context, cmd CreateItemCommand) (domain.CatalogItem, error)
	EditItem(ctx context.Context, cmd EditItemCommand) (domain.CatalogItem, error)
	ListItems(ctx context.Context, onlyActive bool) ([]domain.CatalogItem, error)
}

// --- Canjes (spec 007) ---

type RedeemCommand struct {
	RedeemedBy    domain.MemberID
	CatalogItemID domain.ItemID
	Amount        int
	Note          string
}

type RedemptionFilter struct {
	MemberID *domain.MemberID
}

type RedemptionUseCase interface {
	Redeem(ctx context.Context, cmd RedeemCommand) (domain.Redemption, error)
	Void(ctx context.Context, id domain.RedemptionID, by domain.MemberID) (domain.Redemption, error)
	List(ctx context.Context, f RedemptionFilter) ([]domain.Redemption, error)
}
