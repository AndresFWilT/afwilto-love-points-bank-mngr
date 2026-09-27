// Package service orquesta: pide datos, llama al dominio, persiste. No decide reglas.
package service

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type PointsService struct {
	entries out.EntryRepository
	members out.MemberRepository
	clock   out.Clock
	ids     out.IDGenerator
}

var _ in.PointsUseCase = (*PointsService)(nil)

func New(entries out.EntryRepository, members out.MemberRepository, clock out.Clock, ids out.IDGenerator) *PointsService {
	return &PointsService{entries: entries, members: members, clock: clock, ids: ids}
}

func (s *PointsService) Award(ctx context.Context, cmd in.AwardCommand) (domain.PointEntry, error) {
	ok, err := s.members.Exists(ctx, cmd.AwardedTo)
	if err != nil {
		return domain.PointEntry{}, err
	}
	if !ok {
		return domain.PointEntry{}, domain.ErrMemberNotFound
	}

	entry, err := domain.NewPointEntry( // <- aquí viven las reglas
		s.ids.NewEntryID(), cmd.AwardedTo, cmd.AwardedBy,
		cmd.Amount, cmd.Reason, cmd.OccurredAt, s.clock.Now(),
	)
	if err != nil {
		return domain.PointEntry{}, err
	}

	if err := s.entries.Save(ctx, entry); err != nil {
		return domain.PointEntry{}, err
	}
	return entry, nil
}

func (s *PointsService) Edit(ctx context.Context, cmd in.EditCommand) (domain.PointEntry, error) {
	current, err := s.entries.FindByID(ctx, cmd.EntryID)
	if err != nil {
		return domain.PointEntry{}, err
	}

	edited, rev, err := current.Edit(
		s.ids.NewRevisionID(), cmd.Amount, cmd.Reason, cmd.OccurredAt,
		s.clock.Now(), cmd.EditedBy,
	)
	if err != nil {
		return domain.PointEntry{}, err
	}

	if err := s.entries.Update(ctx, edited, rev); err != nil {
		return domain.PointEntry{}, err
	}
	return edited, nil
}

func (s *PointsService) Void(ctx context.Context, id domain.EntryID, by domain.MemberID) (domain.PointEntry, error) {
	current, err := s.entries.FindByID(ctx, id)
	if err != nil {
		return domain.PointEntry{}, err
	}

	voided, err := current.Void(s.clock.Now(), by)
	if err != nil {
		return domain.PointEntry{}, err
	}

	if err := s.entries.UpdateStatus(ctx, voided); err != nil {
		return domain.PointEntry{}, err
	}
	return voided, nil
}

func (s *PointsService) Ledger(ctx context.Context, f in.LedgerFilter) (in.LedgerPage, error) {
	if f.PageSize <= 0 {
		f.PageSize = in.DefaultPageSize
	}
	if f.PageSize > in.MaxPageSize {
		f.PageSize = in.MaxPageSize
	}

	rows, err := s.entries.List(ctx, f)
	if err != nil {
		return in.LedgerPage{}, err
	}

	// El repositorio devuelve hasta PageSize+1: la fila extra revela que hay más y
	// se convierte en el cursor de la página siguiente (spec 002, CA-3).
	page := in.LedgerPage{Entries: rows}
	if len(rows) > f.PageSize {
		last := rows[f.PageSize-1]
		page.Entries = rows[:f.PageSize]
		page.NextCursor = &in.Cursor{OccurredAt: last.OccurredAt, ID: last.ID}
	}
	return page, nil
}

func (s *PointsService) Balances(ctx context.Context) ([]domain.Balance, error) {
	return s.members.Balances(ctx)
}

func (s *PointsService) Members(ctx context.Context) ([]domain.Member, error) {
	return s.members.All(ctx)
}
