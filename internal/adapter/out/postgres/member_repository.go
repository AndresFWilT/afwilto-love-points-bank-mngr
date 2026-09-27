package postgres

import (
	"context"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type MemberRepository struct {
	db *DB
}

var _ out.MemberRepository = (*MemberRepository)(nil)

func NewMemberRepository(db *DB) *MemberRepository { return &MemberRepository{db: db} }

func (r *MemberRepository) Exists(ctx context.Context, id domain.MemberID) (bool, error) {
	var exists bool
	err := r.db.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM members WHERE id = $1)`, id.String()).Scan(&exists)
	return exists, err
}

func (r *MemberRepository) All(ctx context.Context) ([]domain.Member, error) {
	rows, err := r.db.pool.Query(ctx, `SELECT id, display_name, color FROM members ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Member
	for rows.Next() {
		var id, name, color string
		if err := rows.Scan(&id, &name, &color); err != nil {
			return nil, err
		}
		out = append(out, domain.Member{ID: domain.MemberID(id), DisplayName: name, Color: color})
	}
	return out, rows.Err()
}

// Balances lee la vista member_balances (migración 000004). El LEFT JOIN de la vista
// garantiza que un titular sin entradas aparezca con 0 (spec 005, CA-3).
func (r *MemberRepository) Balances(ctx context.Context) ([]domain.Balance, error) {
	rows, err := r.db.pool.Query(ctx, `
		SELECT member_id, display_name, points_earned, entries_count, last_entry_at,
		       points_redeemed, points_available
		  FROM member_balances
		 ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Balance
	for rows.Next() {
		var (
			id, name           string
			earned             int
			count              int
			lastEntryAt        *time.Time
			redeemed, available int
		)
		if err := rows.Scan(&id, &name, &earned, &count, &lastEntryAt, &redeemed, &available); err != nil {
			return nil, err
		}
		out = append(out, domain.Balance{
			MemberID:       domain.MemberID(id),
			DisplayName:    name,
			PointsEarned:   earned,
			EntriesCount:   count,
			LastEntryAt:    lastEntryAt,
			PointsRedeemed: redeemed,
			PointsAvailable: available,
		})
	}
	return out, rows.Err()
}
