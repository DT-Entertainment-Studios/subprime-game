package game

import "testing"

func TestAdvanceTurnIncrementsTurn(t *testing.T) {
	g, _ := newTestGame(t, 3)
	if got := g.Turn; got != 1 {
		t.Fatalf("initial Turn = %d, want 1", got)
	}
	g.advanceTurn()
	if got := g.Turn; got != 2 {
		t.Errorf("Turn after first advanceTurn = %d, want 2", got)
	}
	g.advanceTurn()
	if got := g.Turn; got != 3 {
		t.Errorf("Turn after second advanceTurn = %d, want 3", got)
	}
}

func TestBankruptStampsEliminatedAt(t *testing.T) {
	g, _ := newTestGame(t, 3)
	// Current is 0; bankrupting the non-current seat 1 keeps two active players
	// so the game does not finish and the turn counter stays put.
	g.bankrupt(1, map[int]int{})
	if got := g.Players[1].EliminatedAt; got != g.Turn {
		t.Errorf("EliminatedAt = %d, want current Turn %d", got, g.Turn)
	}
	if got := g.Turn; got != 1 {
		t.Errorf("Turn = %d, want 1 (bankruptcy must not advance the turn)", got)
	}
}

func TestSameTurnBankruptciesShareEliminatedAt(t *testing.T) {
	g, _ := newTestGame(t, 4)
	// Current is 0; bankrupt two non-current seats, leaving 0 and 3 active so
	// the game continues. Both bankruptcies happen within the same turn.
	g.bankrupt(1, map[int]int{})
	g.bankrupt(2, map[int]int{})
	a, b := g.Players[1].EliminatedAt, g.Players[2].EliminatedAt
	if a != b {
		t.Errorf("EliminatedAt = %d/%d, want equal for the same turn", a, b)
	}
}

func TestLaterTurnBankruptcyHasGreaterEliminatedAt(t *testing.T) {
	g, _ := newTestGame(t, 4)
	g.bankrupt(1, map[int]int{})
	first := g.Players[1].EliminatedAt
	g.advanceTurn()
	g.bankrupt(3, map[int]int{})
	second := g.Players[3].EliminatedAt
	if second <= first {
		t.Errorf("later EliminatedAt = %d, want > first %d", second, first)
	}
}
