package game

import "testing"

func TestRentPaidToPerOpponent(t *testing.T) {
	g, _ := newTestGame(t, 4)
	tenant := 2
	owner0, owner1 := 0, 1
	g.Tiles[11].Owner = owner0 // Gartenstraße (rosa)
	g.Tiles[21].Owner = owner1 // rot street
	g.Tiles[12].Owner = 3      // Elektrizitätswerk, lets chargeRent trigger a utility share

	g.chargeRent(tenant, owner0, 11, 100)
	g.chargeRent(tenant, owner1, 21, 50)

	rentTo := g.Players[tenant].Stats.RentPaidTo
	if got := rentTo[owner0]; got != 100 {
		t.Errorf("RentPaidTo[%d] = %d, want 100", owner0, got)
	}
	if got := rentTo[owner1]; got != 50 {
		t.Errorf("RentPaidTo[%d] = %d, want 50", owner1, got)
	}
	sum := 0
	for _, amount := range rentTo {
		sum += amount
	}
	if got := g.Players[tenant].Stats.RentPaid; sum != 150 || got != 150 {
		t.Errorf("sum(RentPaidTo) = %d, RentPaid = %d, want both 150", sum, got)
	}
	// The utility share is paid by the owner to player 3, never by the tenant,
	// so it must not appear in the tenant's rent breakdown.
	if got := rentTo[3]; got != 0 {
		t.Errorf("RentPaidTo[3] (utility owner) = %d, want 0", got)
	}
	if len(rentTo) != 2 {
		t.Errorf("RentPaidTo = %v, want exactly the two direct rent owners", rentTo)
	}
}
