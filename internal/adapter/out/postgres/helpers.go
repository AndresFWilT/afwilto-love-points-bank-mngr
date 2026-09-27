package postgres

import (
	"strconv"
	"time"
)

func itoa(n int) string { return strconv.Itoa(n) }

// zeroTimes devuelve tres time.Time cero para declararlos juntos en los Scan.
func zeroTimes() (time.Time, time.Time, time.Time) {
	return time.Time{}, time.Time{}, time.Time{}
}
