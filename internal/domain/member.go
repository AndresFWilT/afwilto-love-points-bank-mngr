package domain

import "time"

// MemberID identifica a un titular.
type MemberID string

func (m MemberID) String() string { return string(m) }

// Member es un titular de la libreta. En v1 hay exactamente dos, creados por seed.
type Member struct {
	ID          MemberID
	DisplayName string
	Color       string
}

// Balance es el saldo de un titular (RD-8). En fase 2 se le resta la suma de canjes:
// PointsAvailable = PointsEarned − PointsRedeemed, nunca negativo.
type Balance struct {
	MemberID       MemberID
	DisplayName    string
	PointsEarned   int
	EntriesCount   int
	LastEntryAt    *time.Time // nulo si el titular no tiene entradas activas
	PointsRedeemed int        // fase 2: suma de canjes activos
	PointsAvailable int       // fase 2: earned − redeemed
}
