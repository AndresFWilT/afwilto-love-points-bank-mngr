package out

import (
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// Clock abstrae el tiempo para que los tests sean deterministas.
type Clock interface{ Now() time.Time }

// IDGenerator produce identificadores nuevos. La app construye entidades completas sin
// ir a la base primero.
type IDGenerator interface {
	NewEntryID() domain.EntryID
	NewRevisionID() domain.EntryID
	NewItemID() domain.ItemID
	NewRedemptionID() domain.RedemptionID
}
