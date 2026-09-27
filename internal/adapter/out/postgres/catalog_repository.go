package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

type CatalogRepository struct {
	db *DB
}

var _ out.CatalogRepository = (*CatalogRepository)(nil)

func NewCatalogRepository(db *DB) *CatalogRepository { return &CatalogRepository{db: db} }

// El `tier` no se lee de la base: se deriva en el dominio. La columna generada de la base
// es solo la última línea de defensa, no la fuente que consume la app.
const catalogColumns = `id, name, description, cost, is_active, created_at, updated_at`

func scanItem(row pgx.Row) (domain.CatalogItem, error) {
	var (
		id, name, description string
		cost                  int
		isActive              bool
	)
	createdAt, updatedAt, _ := zeroTimes()
	if err := row.Scan(&id, &name, &description, &cost, &isActive, &createdAt, &updatedAt); err != nil {
		return domain.CatalogItem{}, err
	}
	return domain.RehydrateCatalogItem(domain.ItemID(id), name, description, cost, isActive, createdAt, updatedAt), nil
}

func (r *CatalogRepository) Save(ctx context.Context, item domain.CatalogItem) error {
	_, err := r.db.pool.Exec(ctx, `
		INSERT INTO catalog_items (id, name, description, cost, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		item.ID.String(), item.Name, item.Description, item.Cost, item.IsActive, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func (r *CatalogRepository) Update(ctx context.Context, item domain.CatalogItem) error {
	_, err := r.db.pool.Exec(ctx, `
		UPDATE catalog_items
		   SET name = $2, description = $3, cost = $4, is_active = $5, updated_at = $6
		 WHERE id = $1`,
		item.ID.String(), item.Name, item.Description, item.Cost, item.IsActive, item.UpdatedAt,
	)
	return err
}

func (r *CatalogRepository) FindByID(ctx context.Context, id domain.ItemID) (domain.CatalogItem, error) {
	row := r.db.pool.QueryRow(ctx, `SELECT `+catalogColumns+` FROM catalog_items WHERE id = $1`, id.String())
	item, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CatalogItem{}, domain.ErrItemNotFound
	}
	return item, err
}

func (r *CatalogRepository) List(ctx context.Context, onlyActive bool) ([]domain.CatalogItem, error) {
	q := `SELECT ` + catalogColumns + ` FROM catalog_items`
	if onlyActive {
		q += ` WHERE is_active`
	}
	q += ` ORDER BY cost, created_at`

	rows, err := r.db.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.CatalogItem
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
