package domain

import "errors"

// Errores tipados del dominio. El adaptador HTTP los mapea a status con errors.Is,
// nunca comparando strings. Cada uno corresponde a una invariante RD-x.
var (
	ErrAmountOutOfRange = errors.New("amount out of range")   // RD-1
	ErrReasonTooShort   = errors.New("reason too short")      // RD-2
	ErrSelfAward        = errors.New("self award not allowed") // RD-3
	ErrFutureDate       = errors.New("occurred_at is in the future") // RD-4
	ErrNotTheAwarder    = errors.New("only the awarder can modify")  // RD-5
	ErrEntryVoided      = errors.New("entry is voided")       // RD-6
	ErrEntryNotFound    = errors.New("entry not found")
	ErrMemberNotFound   = errors.New("member not found")

	// Fase 2 — catálogo (RD-CAT-x) y canjes (RD-CAN-x)
	ErrCostNotPositive    = errors.New("cost must be positive")           // RD-CAT-1
	ErrNameTooShort       = errors.New("name too short")                  // RD-CAT-2
	ErrItemInactive       = errors.New("catalog item is inactive")        // RD-CAN-3
	ErrItemNotFound       = errors.New("catalog item not found")
	ErrAmountNotPositive  = errors.New("amount must be positive")         // RD-CAN-1
	ErrNoteTooShort       = errors.New("note too short")                  // RD-CAN-9
	ErrInsufficientBalance = errors.New("insufficient available balance") // RD-CAN-2
	ErrRedemptionNotFound = errors.New("redemption not found")
	ErrNotTheRedeemer     = errors.New("only the redeemer can modify")    // RD-CAN-6
	ErrRedemptionVoided   = errors.New("redemption is voided")            // RD-CAN-7
)

// Code devuelve el código de dominio estable para el campo `code` del problem+json.
func Code(err error) string {
	switch {
	case errors.Is(err, ErrAmountOutOfRange):
		return "amount_out_of_range"
	case errors.Is(err, ErrReasonTooShort):
		return "reason_too_short"
	case errors.Is(err, ErrSelfAward):
		return "self_award"
	case errors.Is(err, ErrFutureDate):
		return "future_date"
	case errors.Is(err, ErrNotTheAwarder):
		return "not_the_awarder"
	case errors.Is(err, ErrEntryVoided):
		return "entry_voided"
	case errors.Is(err, ErrEntryNotFound):
		return "entry_not_found"
	case errors.Is(err, ErrMemberNotFound):
		return "member_not_found"
	case errors.Is(err, ErrCostNotPositive):
		return "cost_not_positive"
	case errors.Is(err, ErrNameTooShort):
		return "name_too_short"
	case errors.Is(err, ErrItemInactive):
		return "item_inactive"
	case errors.Is(err, ErrItemNotFound):
		return "item_not_found"
	case errors.Is(err, ErrAmountNotPositive):
		return "amount_not_positive"
	case errors.Is(err, ErrNoteTooShort):
		return "note_too_short"
	case errors.Is(err, ErrInsufficientBalance):
		return "insufficient_balance"
	case errors.Is(err, ErrRedemptionNotFound):
		return "redemption_not_found"
	case errors.Is(err, ErrNotTheRedeemer):
		return "not_the_redeemer"
	case errors.Is(err, ErrRedemptionVoided):
		return "redemption_voided"
	default:
		return "internal"
	}
}
