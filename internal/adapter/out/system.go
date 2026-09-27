// Package out (adapter) contiene adaptadores de salida triviales: reloj real y
// generador de ids con UUID.
package out

import (
	"time"

	"github.com/google/uuid"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// SystemClock usa el reloj de la máquina, siempre en UTC.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }

// UUIDGenerator produce identificadores UUIDv4 desde la aplicación (no la base).
type UUIDGenerator struct{}

func (UUIDGenerator) NewEntryID() domain.EntryID    { return domain.EntryID(uuid.NewString()) }
func (UUIDGenerator) NewRevisionID() domain.EntryID { return domain.EntryID(uuid.NewString()) }
func (UUIDGenerator) NewItemID() domain.ItemID      { return domain.ItemID(uuid.NewString()) }
func (UUIDGenerator) NewRedemptionID() domain.RedemptionID {
	return domain.RedemptionID(uuid.NewString())
}
