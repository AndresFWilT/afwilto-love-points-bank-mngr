//go:build e2e

// Camino feliz completo contra el compose levantado (make test-e2e).
// Requiere API_URL apuntando a la api viva (por defecto http://localhost:8080).
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	memA = "11111111-1111-4111-8111-111111111111"
	memB = "22222222-2222-4222-8222-222222222222"
)

func baseURL() string {
	if v := os.Getenv("API_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func TestSmoke_AwardListVoidBalance(t *testing.T) {
	base := baseURL()
	client := &http.Client{Timeout: 10 * time.Second}

	// 1. abonar
	body := map[string]any{
		"awarded_to":  memA,
		"amount":      3,
		"reason":      "porque me contó del susto de anoche",
		"occurred_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/entries", bytes.NewReader(buf))
	req.Header.Set("X-Member-Id", memB)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("abonar: esperaba 201, obtuve %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 2. el libro lo lista
	resp, err = client.Get(base + "/api/v1/entries")
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Entries []entryRef `json:"entries"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&page)
	resp.Body.Close()
	if !contains(page.Entries, created.ID) {
		t.Fatal("la entrada creada no aparece en el libro")
	}

	// 3. el saldo de A refleja los 3
	if got := balanceOf(t, client, base, memA); got < 3 {
		t.Fatalf("saldo de A debería ser >= 3, obtuve %d", got)
	}

	// 4. anular y verificar que baja
	before := balanceOf(t, client, base, memA)
	req, _ = http.NewRequest(http.MethodPost, base+"/api/v1/entries/"+created.ID+"/void", nil)
	req.Header.Set("X-Member-Id", memB)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anular: esperaba 200, obtuve %d", resp.StatusCode)
	}
	resp.Body.Close()
	if after := balanceOf(t, client, base, memA); after != before-3 {
		t.Fatalf("tras anular, el saldo debía bajar 3: antes %d después %d", before, after)
	}
}

func balanceOf(t *testing.T, c *http.Client, base, member string) int {
	t.Helper()
	resp, err := c.Get(base + "/api/v1/balances")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var bals []struct {
		MemberID     string `json:"member_id"`
		PointsEarned int    `json:"points_earned"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&bals)
	for _, b := range bals {
		if b.MemberID == member {
			return b.PointsEarned
		}
	}
	t.Fatal(fmt.Sprintf("no se encontró el saldo de %s", member))
	return 0
}

type entryRef struct {
	ID string `json:"id"`
}

func contains(xs []entryRef, id string) bool {
	for _, x := range xs {
		if x.ID == id {
			return true
		}
	}
	return false
}
