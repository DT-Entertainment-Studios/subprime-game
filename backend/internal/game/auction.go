package game

import "time"

// auctionBidStep is the granularity of auction bids and the minimum first bid.
const auctionBidStep = 20

// startAuction puts an unowned tile up for auction.
func (g *Game) startAuction(tile int) {
	g.Pending = nil
	g.Auction = &Auction{
		Tile:   tile,
		Bidder: Bank,
		MinBid: auctionBidStep,
		EndsAt: g.clock().Add(g.auctionDuration),
	}
	g.TurnPhase = TurnAuction
	g.logf("auction_start", g.Current, 0, "%s wird versteigert (Mindestgebot %d GE).", g.tile(tile).Name, auctionBidStep)
}

// Bid places an auction bid. Bids must be positive multiples of 20 and exceed
// the current bid; the first bid is not required to reach the catalog price.
func (g *Game) Bid(player, amount int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if g.Auction == nil {
		return g.fail("wrong_phase", "Es läuft keine Auktion.")
	}
	if !g.active(player) {
		return g.fail("not_permitted", "Du bist nicht mehr im Spiel.")
	}
	a := g.Auction
	if player == a.Bidder {
		return g.fail("invalid_bid", "Du bist bereits Höchstbietender.")
	}
	if amount <= 0 || amount%auctionBidStep != 0 {
		return g.fail("invalid_bid", "Gebote müssen durch 20 teilbar sein.")
	}
	if a.Bid == 0 {
		if amount < a.MinBid {
			return g.fail("invalid_bid", "Das Mindestgebot beträgt %d GE.", a.MinBid)
		}
	} else if amount <= a.Bid {
		return g.fail("invalid_bid", "Das Gebot muss über %d GE liegen.", a.Bid)
	}
	if g.Players[player].Cash < amount {
		return g.fail("insufficient_cash", "Du kannst dieses Gebot nicht bezahlen.")
	}
	now := g.clock()
	a.Bid = amount
	a.Bidder = player
	a.History = append(a.History, Bid{Player: player, Amount: amount, At: now})
	a.EndsAt = now.Add(g.auctionDuration)
	g.logf("bid", player, amount, "%s bietet %d GE für %s.", g.Players[player].Name, amount, g.tile(a.Tile).Name)
	return nil
}

// resolveAuction settles an expired auction.
func (g *Game) resolveAuction() {
	a := g.Auction
	if a == nil {
		return
	}
	g.Auction = nil
	if a.Bidder == Bank {
		g.logf("auction_end", Bank, 0, "Niemand bietet auf %s; das Feld bleibt bei der Bank.", g.tile(a.Tile).Name)
		g.completeTurn()
		return
	}
	winner := a.Bidder
	price := a.Bid
	doubled := g.wouldCompleteGroup(winner, a.Tile)
	if doubled {
		price = a.Bid * 120 / 100
	}
	pl := g.Players[winner]
	if pl.Cash < price {
		g.logf("auction_end", winner, 0, "%s kann den Zuschlag nicht bezahlen.", pl.Name)
		g.completeTurn()
		return
	}
	pl.Cash -= price
	g.Tiles[a.Tile] = TileState{Owner: winner}
	pl.Stats.PropertiesBought++
	g.logf("auction_end", winner, price, "%s ersteigert %s für %d GE.", pl.Name, g.tile(a.Tile).Name, price)
	if doubled {
		g.logf("double_price", winner, price, "Die Auktion vervollständigt die Farbgruppe: 120 %% des Gebots werden fällig.")
	}
	g.completeTurn()
}

// Tick resolves expired auctions and runs at most one autopilot action. It
// reports whether anything changed and a broadcast is needed.
func (g *Game) Tick(now time.Time) bool {
	if g.Phase != PhasePlaying {
		return false
	}
	return g.tickAutopilot(now)
}
