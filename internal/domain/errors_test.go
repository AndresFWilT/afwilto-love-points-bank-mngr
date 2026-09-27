package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

func TestCode_MapsEveryDomainError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{domain.ErrAmountOutOfRange, "amount_out_of_range"},
		{domain.ErrReasonTooShort, "reason_too_short"},
		{domain.ErrSelfAward, "self_award"},
		{domain.ErrFutureDate, "future_date"},
		{domain.ErrNotTheAwarder, "not_the_awarder"},
		{domain.ErrEntryVoided, "entry_voided"},
		{domain.ErrEntryNotFound, "entry_not_found"},
		{domain.ErrMemberNotFound, "member_not_found"},
		{domain.ErrCostNotPositive, "cost_not_positive"},
		{domain.ErrNameTooShort, "name_too_short"},
		{domain.ErrItemInactive, "item_inactive"},
		{domain.ErrItemNotFound, "item_not_found"},
		{domain.ErrAmountNotPositive, "amount_not_positive"},
		{domain.ErrNoteTooShort, "note_too_short"},
		{domain.ErrInsufficientBalance, "insufficient_balance"},
		{domain.ErrRedemptionNotFound, "redemption_not_found"},
		{domain.ErrNotTheRedeemer, "not_the_redeemer"},
		{domain.ErrRedemptionVoided, "redemption_voided"},
		{errors.New("otro"), "internal"},
		{fmt.Errorf("envuelto: %w", domain.ErrSelfAward), "self_award"},
	}
	for _, tc := range cases {
		if got := domain.Code(tc.err); got != tc.want {
			t.Fatalf("Code(%v) = %q, esperaba %q", tc.err, got, tc.want)
		}
	}
}

func TestIDStringers(t *testing.T) {
	if domain.MemberID("m").String() != "m" {
		t.Fatal("MemberID.String")
	}
	if domain.EntryID("e").String() != "e" {
		t.Fatal("EntryID.String")
	}
	if domain.ItemID("i").String() != "i" {
		t.Fatal("ItemID.String")
	}
	if domain.RedemptionID("r").String() != "r" {
		t.Fatal("RedemptionID.String")
	}
}
