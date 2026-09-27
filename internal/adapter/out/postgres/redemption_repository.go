package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type RedemptionRepository struct {
	db *DB
}

var _ out.RedemptionRepository = (*RedemptionRepository)(nil)

func NewRedemptionRepository(db *DB) *RedemptionRepository { return &RedemptionRepository{db: db} }

const redemptionColumns = `id, member_id, catalog_item_id, cost_at_redemption, note, status, redeemed_at, updated_at`

func scanRedemption(row pgx.Row) (domain.Redemption, error) {
	var (
		id, member, item, note, status string
		cost                           int
	)
	redeemedAt, updatedAt, _ := zeroTimes()
	if err := row.Scan(&id, &member, &item, &cost, &note, &status, &redeemedAt, &updatedAt); err != nil {
		return domain.Redemption{}, err
	}
	return domain.RehydrateRedemption(
		domain.RedemptionID(id), domain.MemberID(member), domain.ItemID(item),
		cost, note, domain.EntryStatus(status), redeemedAt, updatedAt,
	), nil
}

// RedeemTx es donde vive la garantía anti-sobregiro (RD-CAN-2, RD-CAN-8). Dentro de una
// transacción: bloquea la fila del titular (SELECT ... FOR UPDATE) para serializar los
// canjes concurrentes del mismo titular, recalcula el disponible, y solo inserta si
// alcanza. Dos canjes simultáneos: el primero toma el lock, el segundo espera y ve el
// saldo ya reducido.
func (r *RedemptionRepository) RedeemTx(ctx context.Context, red domain.Redemption) error {
	return r.db.withinTx(ctx, func(tx pgx.Tx) error {
		// Serializa por titular.
		var locked string
		if err := tx.QueryRow(ctx, `SELECT id FROM members WHERE id = $1 FOR UPDATE`,
			red.MemberID.String()).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrMemberNotFound
			}
			return err
		}

		// Disponible = ganado (entradas activas a su favor) − canjeado (canjes activos).
		var available int
		if err := tx.QueryRow(ctx, `
			SELECT
			  COALESCE((SELECT SUM(amount) FROM point_entries
			            WHERE awarded_to = $1 AND status = 'active'), 0)
			- COALESCE((SELECT SUM(cost_at_redemption) FROM redemptions
			            WHERE member_id = $1 AND status = 'active'), 0)`,
			red.MemberID.String()).Scan(&available); err != nil {
			return err
		}

		if red.Cost > available {
			return domain.ErrInsufficientBalance
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO redemptions (id, member_id, catalog_item_id, cost_at_redemption, note, status, redeemed_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			red.ID.String(), red.MemberID.String(), red.CatalogItemID.String(),
			red.Cost, red.Note, string(red.Status), red.RedeemedAt, red.UpdatedAt,
		)
		return err
	})
}

func (r *RedemptionRepository) UpdateStatus(ctx context.Context, red domain.Redemption) error {
	_, err := r.db.pool.Exec(ctx, `
		UPDATE redemptions SET status = $2, updated_at = $3 WHERE id = $1`,
		red.ID.String(), string(red.Status), red.UpdatedAt,
	)
	return err
}

func (r *RedemptionRepository) FindByID(ctx context.Context, id domain.RedemptionID) (domain.Redemption, error) {
	row := r.db.pool.QueryRow(ctx, `SELECT `+redemptionColumns+` FROM redemptions WHERE id = $1`, id.String())
	red, err := scanRedemption(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Redemption{}, domain.ErrRedemptionNotFound
	}
	return red, err
}

func (r *RedemptionRepository) List(ctx context.Context, f in.RedemptionFilter) ([]domain.Redemption, error) {
	q := `SELECT ` + redemptionColumns + ` FROM redemptions WHERE 1=1`
	args := []any{}
	if f.MemberID != nil {
		args = append(args, f.MemberID.String())
		q += ` AND member_id = $1`
	}
	q += ` ORDER BY redeemed_at DESC, id DESC`

	rows, err := r.db.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Redemption
	for rows.Next() {
		red, err := scanRedemption(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, red)
	}
	return out, rows.Err()
}
