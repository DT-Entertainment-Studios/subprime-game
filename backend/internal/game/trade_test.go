package game

import (
	"bytes"
	"encoding/json"
	"testing"
)

// assertEmptyTileSlices verifies that a marshalled trade carries both tile
// lists as empty JSON arrays rather than null, which used to crash clients.
func assertEmptyTileSlices(t *testing.T, label string, offer TradeOffer) {
	t.Helper()
	data, err := json.Marshal(offer)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", label, err)
	}
	for _, field := range []string{`"offerTiles":[]`, `"requestTiles":[]`} {
		if !bytes.Contains(data, []byte(field)) {
			t.Errorf("%s missing %s in %s", label, field, data)
		}
	}
	if bytes.Contains(data, []byte("null")) {
		t.Errorf("%s contains null in %s", label, data)
	}
}

func TestTradeEmptyTileListsSerializeAsArrays(t *testing.T) {
	g, _ := newTestGame(t, 2)
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferCash: 10})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	var offer TradeOffer
	for _, tr := range g.Trades {
		if tr.ID == id {
			offer = *tr
		}
	}
	assertEmptyTileSlices(t, "offer", offer)

	var snapshot TradeOffer
	for _, tr := range g.State().Trades {
		if tr.ID == id {
			snapshot = tr
		}
	}
	assertEmptyTileSlices(t, "snapshot", snapshot)
}

func TestTradeCashOnlyCanBeAccepted(t *testing.T) {
	g, _ := newTestGame(t, 2)
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferCash: 10})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if err := g.RespondTrade(1, id, true); err != nil {
		t.Fatalf("RespondTrade() error = %v", err)
	}
	if g.Players[0].Cash != 1490 || g.Players[1].Cash != 1510 {
		t.Errorf("cash = %d/%d, want 1490/1510", g.Players[0].Cash, g.Players[1].Cash)
	}
}

func TestTradeWithCompletelyEmptySideCanBeAccepted(t *testing.T) {
	g, _ := newTestGame(t, 2)
	// Player 0 gives nothing at all; player 1 pays 5 for the privilege.
	id, err := g.ProposeTrade(0, TradeInput{To: 1, RequestCash: 5})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if err := g.RespondTrade(1, id, true); err != nil {
		t.Fatalf("RespondTrade() error = %v", err)
	}
	if g.Players[0].Cash != 1505 || g.Players[1].Cash != 1495 {
		t.Errorf("cash = %d/%d, want 1505/1495", g.Players[0].Cash, g.Players[1].Cash)
	}
}

func TestTradeCounterWithEmptyTileListsSerializesAsArrays(t *testing.T) {
	g, _ := newTestGame(t, 2)
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferCash: 10})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	counterID, err := g.ProposeTrade(1, TradeInput{To: 0, RequestCash: 10, CounterTo: id})
	if err != nil {
		t.Fatalf("ProposeTrade() counter error = %v", err)
	}
	var counter TradeOffer
	for _, tr := range g.State().Trades {
		if tr.ID == counterID {
			counter = tr
		}
	}
	assertEmptyTileSlices(t, "counter", counter)
}
