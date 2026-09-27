package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

// EntryID identifica una entrada del libro.
type EntryID string

func (e EntryID) String() string { return string(e) }

// EntryStatus es el estado de una entrada. Nada se borra: se anula (RD-Void-1).
type EntryStatus string

const (
	StatusActive EntryStatus = "active"
	StatusVoided EntryStatus = "voided"
)

const (
	MinAmount     = 1  // RD-1
	MaxAmount     = 5  // RD-1
	MinReasonRune = 10 // RD-2 (caracteres útiles tras recortar)
)

// PointEntry es un movimiento del libro: un abono de puntitos con su justificación.
type PointEntry struct {
	ID         EntryID
	AwardedTo  MemberID
	AwardedBy  MemberID
	Amount     int
	Reason     string
	OccurredAt time.Time
	Status     EntryStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Revision documenta el estado ANTERIOR de una entrada tras editarla (RD-7).
// Guarda el valor viejo: el nuevo ya vive en la fila.
type Revision struct {
	ID             EntryID
	EntryID        EntryID
	EditedBy       MemberID
	PrevAmount     int
	PrevReason     string
	PrevOccurredAt time.Time
	EditedAt       time.Time
}

// validate aplica RD-1..RD-4 sobre valores propuestos. Es el único lugar donde viven.
func validate(amount int, reason string, occurredAt, now time.Time) error {
	if amount < MinAmount || amount > MaxAmount {
		return ErrAmountOutOfRange // RD-1
	}
	if utf8.RuneCountInString(strings.TrimSpace(reason)) < MinReasonRune {
		return ErrReasonTooShort // RD-2
	}
	if occurredAt.After(now) {
		return ErrFutureDate // RD-4
	}
	return nil
}

// NewPointEntry es el único camino para crear una entrada válida.
func NewPointEntry(id EntryID, to, by MemberID, amount int, reason string,
	occurredAt, now time.Time) (PointEntry, error) {

	if to == by {
		return PointEntry{}, ErrSelfAward // RD-3
	}
	if err := validate(amount, reason, occurredAt, now); err != nil {
		return PointEntry{}, err
	}
	return PointEntry{
		ID:         id,
		AwardedTo:  to,
		AwardedBy:  by,
		Amount:     amount,
		Reason:     strings.TrimSpace(reason),
		OccurredAt: occurredAt,
		Status:     StatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// RehydratePointEntry reconstruye una entrada ya persistida SIN revalidar invariantes:
// los datos guardados son válidos por definición, y revalidarlos rompería la lectura
// del histórico si mañana cambia una regla.
func RehydratePointEntry(id EntryID, to, by MemberID, amount int, reason string,
	occurredAt time.Time, status EntryStatus, createdAt, updatedAt time.Time) PointEntry {
	return PointEntry{
		ID:         id,
		AwardedTo:  to,
		AwardedBy:  by,
		Amount:     amount,
		Reason:     reason,
		OccurredAt: occurredAt,
		Status:     status,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

// Edit devuelve una copia modificada y la revisión que documenta el cambio (RD-5,6,7).
// revID es el id de la revisión a crear (lo provee el generador de ids).
func (e PointEntry) Edit(revID EntryID, amount int, reason string, occurredAt, now time.Time,
	editedBy MemberID) (PointEntry, Revision, error) {

	if e.Status == StatusVoided {
		return PointEntry{}, Revision{}, ErrEntryVoided // RD-6
	}
	if editedBy != e.AwardedBy {
		return PointEntry{}, Revision{}, ErrNotTheAwarder // RD-5
	}
	if err := validate(amount, reason, occurredAt, now); err != nil {
		return PointEntry{}, Revision{}, err
	}

	rev := Revision{
		ID:             revID,
		EntryID:        e.ID,
		EditedBy:       editedBy,
		PrevAmount:     e.Amount,
		PrevReason:     e.Reason,
		PrevOccurredAt: e.OccurredAt,
		EditedAt:       now,
	}

	edited := e
	edited.Amount = amount
	edited.Reason = strings.TrimSpace(reason)
	edited.OccurredAt = occurredAt
	edited.UpdatedAt = now
	return edited, rev, nil
}

// Void marca la entrada como anulada (RD-5, RD-6, RD-Void-1). No borra nada.
func (e PointEntry) Void(now time.Time, by MemberID) (PointEntry, error) {
	if e.Status == StatusVoided {
		return PointEntry{}, ErrEntryVoided // RD-6
	}
	if by != e.AwardedBy {
		return PointEntry{}, ErrNotTheAwarder // RD-5
	}
	voided := e
	voided.Status = StatusVoided
	voided.UpdatedAt = now
	return voided, nil
}
