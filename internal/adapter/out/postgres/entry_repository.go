package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type EntryRepository struct {
	db *DB
}

var _ out.EntryRepository = (*EntryRepository)(nil)

func NewEntryRepository(db *DB) *EntryRepository { return &EntryRepository{db: db} }

const entryColumns = `id, awarded_to, awarded_by, amount, reason, occurred_at, status, created_at, updated_at`

// scanEntry mapea una fila a la entidad. La reconstruye SIN revalidar invariantes.
func scanEntry(row pgx.Row) (domain.PointEntry, error) {
	var (
		id, to, by, status              string
		amount                          int
		reason                          string
		occurredAt, createdAt, updatedAt = zeroTimes()
	)
	if err := row.Scan(&id, &to, &by, &amount, &reason, &occurredAt, &status, &createdAt, &updatedAt); err != nil {
		return domain.PointEntry{}, err
	}
	return domain.RehydratePointEntry(
		domain.EntryID(id), domain.MemberID(to), domain.MemberID(by),
		amount, reason, occurredAt, domain.EntryStatus(status), createdAt, updatedAt,
	), nil
}

func (r *EntryRepository) Save(ctx context.Context, e domain.PointEntry) error {
	_, err := r.db.pool.Exec(ctx, `
		INSERT INTO point_entries (id, awarded_to, awarded_by, amount, reason, occurred_at, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		e.ID.String(), e.AwardedTo.String(), e.AwardedBy.String(), e.Amount, e.Reason,
		e.OccurredAt, string(e.Status), e.CreatedAt, e.UpdatedAt,
	)
	return err
}

// Update escribe entrada + revisión en la MISMA transacción (spec 003, CA-5).
func (r *EntryRepository) Update(ctx context.Context, e domain.PointEntry, rev domain.Revision) error {
	return r.db.withinTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE point_entries
			   SET amount = $2, reason = $3, occurred_at = $4, updated_at = $5
			 WHERE id = $1`,
			e.ID.String(), e.Amount, e.Reason, e.OccurredAt, e.UpdatedAt,
		); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO point_entry_revisions (id, entry_id, edited_by, prev_amount, prev_reason, prev_occurred_at, edited_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			rev.ID.String(), rev.EntryID.String(), rev.EditedBy.String(),
			rev.PrevAmount, rev.PrevReason, rev.PrevOccurredAt, rev.EditedAt,
		)
		return err
	})
}

func (r *EntryRepository) UpdateStatus(ctx context.Context, e domain.PointEntry) error {
	_, err := r.db.pool.Exec(ctx, `
		UPDATE point_entries SET status = $2, updated_at = $3 WHERE id = $1`,
		e.ID.String(), string(e.Status), e.UpdatedAt,
	)
	return err
}

func (r *EntryRepository) FindByID(ctx context.Context, id domain.EntryID) (domain.PointEntry, error) {
	row := r.db.pool.QueryRow(ctx, `SELECT `+entryColumns+` FROM point_entries WHERE id = $1`, id.String())
	e, err := scanEntry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PointEntry{}, domain.ErrEntryNotFound
	}
	return e, err
}

// List devuelve hasta f.PageSize+1 filas ordenadas por RD-L-1 (occurred_at DESC, id DESC),
// respetando filtros y cursor. La fila extra sirve al servicio para calcular NextCursor.
func (r *EntryRepository) List(ctx context.Context, f in.LedgerFilter) ([]domain.PointEntry, error) {
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = in.DefaultPageSize
	}

	q := `SELECT ` + entryColumns + ` FROM point_entries WHERE 1=1`
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + itoa(len(args))
	}

	if f.AwardedTo != nil {
		q += ` AND awarded_to = ` + arg(f.AwardedTo.String())
	}
	if f.Status != nil {
		q += ` AND status = ` + arg(string(*f.Status))
	}
	if f.After != nil {
		// Keyset: filas estrictamente posteriores en (occurred_at DESC, id DESC).
		q += ` AND (occurred_at, id) < (` + arg(f.After.OccurredAt) + `, ` + arg(f.After.ID.String()) + `)`
	}
	q += ` ORDER BY occurred_at DESC, id DESC LIMIT ` + arg(pageSize+1)

	rows, err := r.db.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.PointEntry, 0, pageSize+1)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
