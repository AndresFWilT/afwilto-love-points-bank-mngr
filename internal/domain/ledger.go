package domain

import "sort"

// CalculateBalances computa el saldo de cada titular (RD-8, RD-8b, RD-8c):
// ganado = suma de entradas ACTIVAS a su favor; canjeado = suma de canjes ACTIVOS suyos;
// disponible = ganado − canjeado. Las anuladas no cuentan.
// Función pura: mismo conjunto de entradas y canjes, mismo saldo.
func CalculateBalances(members []Member, entries []PointEntry, redemptions []Redemption) []Balance {
	acc := make(map[MemberID]*Balance, len(members))
	order := make([]MemberID, 0, len(members))
	for _, m := range members {
		acc[m.ID] = &Balance{MemberID: m.ID, DisplayName: m.DisplayName}
		order = append(order, m.ID)
	}

	for _, e := range entries {
		if e.Status != StatusActive {
			continue
		}
		b, ok := acc[e.AwardedTo]
		if !ok {
			continue
		}
		b.PointsEarned += e.Amount
		b.EntriesCount++
		if b.LastEntryAt == nil || e.OccurredAt.After(*b.LastEntryAt) {
			t := e.OccurredAt
			b.LastEntryAt = &t
		}
	}

	for _, r := range redemptions {
		if r.Status != StatusActive {
			continue
		}
		b, ok := acc[r.MemberID]
		if !ok {
			continue
		}
		b.PointsRedeemed += r.Cost
	}

	out := make([]Balance, 0, len(order))
	for _, id := range order {
		b := acc[id]
		b.PointsAvailable = b.PointsEarned - b.PointsRedeemed
		out = append(out, *b)
	}
	return out
}

// SortLedger ordena el libro por occurred_at DESC, desempatando por id DESC (RD-L-1),
// para que el orden sea total y estable.
func SortLedger(entries []PointEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].OccurredAt.Equal(entries[j].OccurredAt) {
			return entries[i].OccurredAt.After(entries[j].OccurredAt)
		}
		return entries[i].ID > entries[j].ID
	})
}
