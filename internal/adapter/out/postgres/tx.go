// Package postgres implementa los puertos de salida con pgx/v5. SQL a mano, sin ORM.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB envuelve el pool de conexiones.
type DB struct {
	pool *pgxpool.Pool
}

func NewDB(pool *pgxpool.Pool) *DB { return &DB{pool: pool} }

// withinTx ejecuta fn dentro de una transacción; si fn devuelve error, revierte.
// Es lo que garantiza que entrada + revisión se escriban juntas (spec 003, CA-5).
func (db *DB) withinTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
