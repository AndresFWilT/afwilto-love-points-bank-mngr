package service

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// RedemptionService orquesta los canjes (spec 007). La regla de saldo (RD-CAN-2) se
// delega a RedeemTx, que la verifica dentro de una transacción a prueba de concurrencia.
type RedemptionService struct {
	redemptions out.RedemptionRepository
	items       out.CatalogRepository
	clock       out.Clock
	ids         out.IDGenerator
}

var _ in.RedemptionUseCase = (*RedemptionService)(nil)

func NewRedemption(redemptions out.RedemptionRepository, items out.CatalogRepository,
	clock out.Clock, ids out.IDGenerator) *RedemptionService {
	return &RedemptionService{redemptions: redemptions, items: items, clock: clock, ids: ids}
}

func (s *RedemptionService) Redeem(ctx context.Context, cmd in.RedeemCommand) (domain.Redemption, error) {
	item, err := s.items.FindByID(ctx, cmd.CatalogItemID)
	if err != nil {
		return domain.Redemption{}, err
	}

	// El dominio valida monto, nota y que el plan esté activo, y congela el costo.
	redemption, err := domain.NewRedemption(
		s.ids.NewRedemptionID(), cmd.RedeemedBy, item, cmd.Amount, cmd.Note, s.clock.Now(),
	)
	if err != nil {
		return domain.Redemption{}, err
	}

	// La verificación autoritativa de saldo ocurre dentro de la transacción.
	if err := s.redemptions.RedeemTx(ctx, redemption); err != nil {
		return domain.Redemption{}, err
	}
	return redemption, nil
}

func (s *RedemptionService) Void(ctx context.Context, id domain.RedemptionID, by domain.MemberID) (domain.Redemption, error) {
	current, err := s.redemptions.FindByID(ctx, id)
	if err != nil {
		return domain.Redemption{}, err
	}
	voided, err := current.Void(s.clock.Now(), by)
	if err != nil {
		return domain.Redemption{}, err
	}
	if err := s.redemptions.UpdateStatus(ctx, voided); err != nil {
		return domain.Redemption{}, err
	}
	return voided, nil
}

func (s *RedemptionService) List(ctx context.Context, f in.RedemptionFilter) ([]domain.Redemption, error) {
	return s.redemptions.List(ctx, f)
}
