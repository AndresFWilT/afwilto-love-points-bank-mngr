// Package in define los puertos de entrada (driving): lo que el mundo exterior le puede
// pedir a la aplicación. Solo tipos del dominio; ni un http.Request ni un DTO.
package in

import (
	"context"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type AwardCommand struct {
	AwardedTo  domain.MemberID
	AwardedBy  domain.MemberID
	Amount     int
	Reason     string
	OccurredAt time.Time
}

type EditCommand struct {
	EntryID    domain.EntryID
	EditedBy   domain.MemberID
	Amount     int
	Reason     string
	OccurredAt time.Time
}

// LedgerFilter describe una consulta al libro con paginación por cursor (spec 002).
type LedgerFilter struct {
	AwardedTo *domain.MemberID
	Status    *domain.EntryStatus
	PageSize  int
	// Cursor decodificado: la última fila de la página anterior. Nil en la primera página.
	After *Cursor
}

// Cursor apunta a una fila concreta para paginar sin saltos ni repeticiones (RD-L-1).
type Cursor struct {
	OccurredAt time.Time
	ID         domain.EntryID
}

// LedgerPage es una página del libro más el cursor para pedir la siguiente.
type LedgerPage struct {
	Entries    []domain.PointEntry
	NextCursor *Cursor
}

type PointsUseCase interface {
	Award(ctx context.Context, cmd AwardCommand) (domain.PointEntry, error)
	Edit(ctx context.Context, cmd EditCommand) (domain.PointEntry, error)
	Void(ctx context.Context, id domain.EntryID, by domain.MemberID) (domain.PointEntry, error)
	Ledger(ctx context.Context, f LedgerFilter) (LedgerPage, error)
	Balances(ctx context.Context) ([]domain.Balance, error)
	Members(ctx context.Context) ([]domain.Member, error)
}

const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)
