package testsupport

import (
	"sync"
	"time"

	"github.com/andresfwilt/banco-de-puntitos/internal/domain"
)

// FixedClock devuelve siempre el mismo instante, para tests deterministas.
type FixedClock struct{ T time.Time }

func (c FixedClock) Now() time.Time { return c.T }

// SeqIDs entrega ids predecibles: entry-1, rev-1, item-1, redemption-1, ...
// Es seguro para uso concurrente (los tests de canje disparan goroutines en paralelo).
type SeqIDs struct {
	mu         sync.Mutex
	entry      int
	rev        int
	item       int
	redemption int
}

func (g *SeqIDs) NewEntryID() domain.EntryID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entry++
	return domain.EntryID(seq("entry", g.entry))
}

func (g *SeqIDs) NewRevisionID() domain.EntryID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.rev++
	return domain.EntryID(seq("rev", g.rev))
}

func (g *SeqIDs) NewItemID() domain.ItemID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.item++
	return domain.ItemID(seq("item", g.item))
}

func (g *SeqIDs) NewRedemptionID() domain.RedemptionID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.redemption++
	return domain.RedemptionID(seq("redemption", g.redemption))
}

func seq(prefix string, n int) string {
	digits := []byte{}
	if n == 0 {
		digits = []byte{'0'}
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return prefix + "-" + string(digits)
}
