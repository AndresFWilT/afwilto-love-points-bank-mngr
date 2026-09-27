package http

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/app/port/in"
	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// Los DTO viven solo en el adaptador. El dominio no sabe qué es JSON.

type entryDTO struct {
	ID         string    `json:"id"`
	AwardedTo  string    `json:"awarded_to"`
	AwardedBy  string    `json:"awarded_by"`
	Amount     int       `json:"amount"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func toEntryDTO(e domain.PointEntry) entryDTO {
	return entryDTO{
		ID:         e.ID.String(),
		AwardedTo:  e.AwardedTo.String(),
		AwardedBy:  e.AwardedBy.String(),
		Amount:     e.Amount,
		Reason:     e.Reason,
		OccurredAt: e.OccurredAt,
		Status:     string(e.Status),
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
	}
}

type awardRequest struct {
	AwardedTo  string    `json:"awarded_to"`
	Amount     int       `json:"amount"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

type editRequest struct {
	Amount     int       `json:"amount"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ledgerPageDTO struct {
	Entries    []entryDTO `json:"entries"`
	NextCursor *string    `json:"next_cursor"`
}

func toLedgerPageDTO(p in.LedgerPage) ledgerPageDTO {
	items := make([]entryDTO, 0, len(p.Entries))
	for _, e := range p.Entries {
		items = append(items, toEntryDTO(e))
	}
	dto := ledgerPageDTO{Entries: items}
	if p.NextCursor != nil {
		c := encodeCursor(*p.NextCursor)
		dto.NextCursor = &c
	}
	return dto
}

type balanceDTO struct {
	MemberID       string     `json:"member_id"`
	DisplayName    string     `json:"display_name"`
	PointsEarned   int        `json:"points_earned"`
	EntriesCount   int        `json:"entries_count"`
	LastEntryAt    *time.Time `json:"last_entry_at"`
	PointsRedeemed int        `json:"points_redeemed"`
	PointsAvailable int       `json:"points_available"`
}

func toBalanceDTO(b domain.Balance) balanceDTO {
	return balanceDTO{
		MemberID:       b.MemberID.String(),
		DisplayName:    b.DisplayName,
		PointsEarned:   b.PointsEarned,
		EntriesCount:   b.EntriesCount,
		LastEntryAt:    b.LastEntryAt,
		PointsRedeemed: b.PointsRedeemed,
		PointsAvailable: b.PointsAvailable,
	}
}

// --- Fase 2: catálogo y canjes ---

type catalogItemDTO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Cost        int       `json:"cost"`
	Tier        string    `json:"tier"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toCatalogItemDTO(i domain.CatalogItem) catalogItemDTO {
	return catalogItemDTO{
		ID:          i.ID.String(),
		Name:        i.Name,
		Description: i.Description,
		Cost:        i.Cost,
		Tier:        string(i.Tier()),
		IsActive:    i.IsActive,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
}

type createItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
}

type editItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
	IsActive    bool   `json:"is_active"`
}

type redemptionDTO struct {
	ID            string    `json:"id"`
	MemberID      string    `json:"member_id"`
	CatalogItemID string    `json:"catalog_item_id"`
	Cost          int       `json:"cost_at_redemption"`
	Note          string    `json:"note"`
	Status        string    `json:"status"`
	RedeemedAt    time.Time `json:"redeemed_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toRedemptionDTO(r domain.Redemption) redemptionDTO {
	return redemptionDTO{
		ID:            r.ID.String(),
		MemberID:      r.MemberID.String(),
		CatalogItemID: r.CatalogItemID.String(),
		Cost:          r.Cost,
		Note:          r.Note,
		Status:        string(r.Status),
		RedeemedAt:    r.RedeemedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

type redeemRequest struct {
	CatalogItemID string `json:"catalog_item_id"`
	Amount        int    `json:"amount"`
	Note          string `json:"note"`
}

type memberDTO struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Color       string `json:"color"`
}

func toMemberDTO(m domain.Member) memberDTO {
	return memberDTO{ID: m.ID.String(), DisplayName: m.DisplayName, Color: m.Color}
}

// encodeCursor produce un cursor opaco base64 de "RFC3339Nano|id" (spec 002).
func encodeCursor(c in.Cursor) string {
	raw := c.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*in.Cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, errBadCursor
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, err
	}
	return &in.Cursor{OccurredAt: t, ID: domain.EntryID(parts[1])}, nil
}
