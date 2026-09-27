package service

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// CatalogService orquesta el catálogo de planes (spec 006). No decide reglas.
type CatalogService struct {
	items out.CatalogRepository
	clock out.Clock
	ids   out.IDGenerator
}

var _ in.CatalogUseCase = (*CatalogService)(nil)

func NewCatalog(items out.CatalogRepository, clock out.Clock, ids out.IDGenerator) *CatalogService {
	return &CatalogService{items: items, clock: clock, ids: ids}
}

func (s *CatalogService) CreateItem(ctx context.Context, cmd in.CreateItemCommand) (domain.CatalogItem, error) {
	item, err := domain.NewCatalogItem(s.ids.NewItemID(), cmd.Name, cmd.Description, cmd.Cost, s.clock.Now())
	if err != nil {
		return domain.CatalogItem{}, err
	}
	if err := s.items.Save(ctx, item); err != nil {
		return domain.CatalogItem{}, err
	}
	return item, nil
}

func (s *CatalogService) EditItem(ctx context.Context, cmd in.EditItemCommand) (domain.CatalogItem, error) {
	current, err := s.items.FindByID(ctx, cmd.ItemID)
	if err != nil {
		return domain.CatalogItem{}, err
	}
	edited, err := current.Edit(cmd.Name, cmd.Description, cmd.Cost, cmd.IsActive, s.clock.Now())
	if err != nil {
		return domain.CatalogItem{}, err
	}
	if err := s.items.Update(ctx, edited); err != nil {
		return domain.CatalogItem{}, err
	}
	return edited, nil
}

func (s *CatalogService) ListItems(ctx context.Context, onlyActive bool) ([]domain.CatalogItem, error) {
	return s.items.List(ctx, onlyActive)
}
