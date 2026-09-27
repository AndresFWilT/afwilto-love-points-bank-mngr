package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// RedemptionID identifica un canje.
type RedemptionID string

func (r RedemptionID) String() string { return string(r) }

// MinNoteRune: el porqué del canje es obligatorio, como la justificación de un abono.
const MinNoteRune = 10 // RD-CAN-9

// Redemption es un canje: gastar puntitos propios en un plan del catálogo. Resta saldo.
type Redemption struct {
	ID            RedemptionID
	MemberID      MemberID
	CatalogItemID ItemID
	Cost          int    // cost_at_redemption, congelado (RD-CAN-4)
	Note          string
	Status        EntryStatus // reutiliza active/voided
	RedeemedAt    time.Time
	UpdatedAt     time.Time
}

// NewRedemption valida RD-CAN-1/3/4/5/9 y congela el costo. La regla de saldo (RD-CAN-2)
// NO va aquí: necesita el disponible, que el servicio calcula dentro de la transacción.
func NewRedemption(id RedemptionID, by MemberID, item CatalogItem, amount int,
	note string, now time.Time) (Redemption, error) {

	if amount < 1 {
		return Redemption{}, ErrAmountNotPositive // RD-CAN-1
	}
	if utf8.RuneCountInString(strings.TrimSpace(note)) < MinNoteRune {
		return Redemption{}, ErrNoteTooShort // RD-CAN-9
	}
	if !item.IsActive {
		return Redemption{}, ErrItemInactive // RD-CAN-3
	}
	return Redemption{
		ID:            id,
		MemberID:      by,
		CatalogItemID: item.ID,
		Cost:          amount, // RD-CAN-4/5: congela el monto gastado (híbrido)
		Note:          strings.TrimSpace(note),
		Status:        StatusActive,
		RedeemedAt:    now,
		UpdatedAt:     now,
	}, nil
}

// Void marca el canje como anulado (RD-CAN-6, RD-CAN-7). No borra; devolver los puntos
// es consecuencia de que la vista solo suma canjes activos.
func (r Redemption) Void(now time.Time, by MemberID) (Redemption, error) {
	if r.Status == StatusVoided {
		return Redemption{}, ErrRedemptionVoided // RD-CAN-7
	}
	if by != r.MemberID {
		return Redemption{}, ErrNotTheRedeemer // RD-CAN-6
	}
	voided := r
	voided.Status = StatusVoided
	voided.UpdatedAt = now
	return voided, nil
}

// RehydrateRedemption reconstruye un canje persistido sin revalidar invariantes.
func RehydrateRedemption(id RedemptionID, member MemberID, item ItemID, cost int, note string,
	status EntryStatus, redeemedAt, updatedAt time.Time) Redemption {
	return Redemption{
		ID:            id,
		MemberID:      member,
		CatalogItemID: item,
		Cost:          cost,
		Note:          note,
		Status:        status,
		RedeemedAt:    redeemedAt,
		UpdatedAt:     updatedAt,
	}
}
