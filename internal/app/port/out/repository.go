// Package out define los puertos de salida (driven): lo que la app necesita del mundo
// exterior. Ningún tipo de pgx, database/sql o net/http aparece aquí, por diseño.
package out

import (
	"context"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type EntryRepository interface {
	Save(ctx context.Context, e domain.PointEntry) error
	// Update escribe la entrada y su revisión en la MISMA transacción (spec 003, CA-5).
	Update(ctx context.Context, e domain.PointEntry, r domain.Revision) error
	// UpdateStatus persiste una anulación (spec 004). Sin revisión: no cambia contenido.
	UpdateStatus(ctx context.Context, e domain.PointEntry) error
	FindByID(ctx context.Context, id domain.EntryID) (domain.PointEntry, error)
	// List devuelve hasta f.PageSize+1 filas ordenadas por RD-L-1; el servicio usa la
	// fila extra para calcular el cursor.
	List(ctx context.Context, f in.LedgerFilter) ([]domain.PointEntry, error)
}

type MemberRepository interface {
	Exists(ctx context.Context, id domain.MemberID) (bool, error)
	All(ctx context.Context) ([]domain.Member, error)
	Balances(ctx context.Context) ([]domain.Balance, error)
}
