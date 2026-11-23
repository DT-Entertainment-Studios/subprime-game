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

// auctionWinPrice returns what a winning bid costs the bidder. Winning a tile
// that completes the bidder's color group costs 120% of the bid (integer
// rounding); doubled reports whether that rule applied. Both the affordability
// check in Bid and the charge in resolveAuction use this single computation.
func (g *Game) auctionWinPrice(bidder, tile, bid int) (price int, doubled bool) {
	price = bid
	if g.wouldCompleteGroup(bidder, tile) {
		price = bid * 120 / 100
		doubled = true
	}
	return price, doubled
}

// Bid places an auction bid. Bids must be positive multiples of 20 and exceed
// the current bid; the first bid is not required to reach the catalog price.
// The limit is the bidder's full liquidation value, not just their cash, measured
// against the effective price: 120% of the bid when winning would complete the
// tile's color group, otherwise the raw bid. A winning bid above cash sends the
// bidder into the debt phase (Spec.md §4.4).
func (g *Game) Bid(player, amount int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if err := g.requireActor(player); err != nil {
		return err
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
	if price, _ := g.auctionWinPrice(player, a.Tile, amount); g.liquidationValue(player) < price {
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

// resolveAuction settles an expired auction. The won tile is held by the Bank
// until any auction debt is settled; registerDebt is the only insolvency gate
// and either pauses the game at the next checkpoint or declares bankruptcy.
func (g *Game) resolveAuction() {
	a := g.Auction
	if a == nil {
		return
	}
	g.Auction = nil
	if a.Bidder == Bank {
		g.logf("auction_end", Bank, 0, "Niemand bietet auf %s; das Feld bleibt bei der Bank.", g.tile(a.Tile).Name)
		if g.debtCheckpoint() {
			return
		}
		g.completeTurn()
		return
	}
	winner := a.Bidder
	price, doubled := g.auctionWinPrice(winner, a.Tile, a.Bid)
	pl := g.Players[winner]
	pl.Cash -= price
	g.holdAuctionWin(winner, a.Tile)
	pl.Stats.PropertiesBought++
	g.logf("auction_end", winner, price, "%s ersteigert %s für %d GE.", pl.Name, g.tile(a.Tile).Name, price)
	if doubled {
		g.logf("double_price", winner, price, "Die Auktion vervollständigt die Farbgruppe: 120 %% des Gebots werden fällig.")
	}
	g.registerDebt(winner, PaymentCause{Code: "auktion", Creditor: "für " + g.tile(a.Tile).Name})
	if g.inDebt(winner) {
		g.logf("auction_debt", winner, -pl.Cash,
			"%s ersteigert %s, das Feld bleibt aber bei der Bank, bis die Auktionsrechnung beglichen ist.",
			pl.Name, g.tile(a.Tile).Name)
	} else {
		g.settlePendingAuctionWin()
	}
	if g.debtCheckpoint() {
		return
	}
	g.completeTurn()
}

// holdAuctionWin records a won tile whose transfer waits for debt settlement.
// It refuses to overwrite an existing record (R4): at most one auction debt can
// be open at a time, so a second held-back win can only be a bug.
func (g *Game) holdAuctionWin(winner, tile int) {
	if g.pendingAuctionWin != nil {
		g.logf("auction_hold_conflict", winner, 0,
			"Warnung: eine weitere Auktionsübergabe für %s ist bereits vorgemerkt.", g.tile(tile).Name)
		return
	}
	g.pendingAuctionWin = &PendingAuctionWin{Tile: tile, Winner: winner}
}

// settlePendingAuctionWin transfers the held-back tile if the winner no longer
// owes money. It is a no-op when there is no pending win, the winner is still
// in debt or no longer active, or the tile is no longer Bank-owned. It logs
// "auction_transfer". Invariant (R3): while a debt is open all other intents are
// blocked, so only the debtor's own liquidation can raise the cash that clears
// the balance here.
func (g *Game) settlePendingAuctionWin() {
	p := g.pendingAuctionWin
	if p == nil {
		return
	}
	if g.inDebt(p.Winner) || !g.active(p.Winner) || g.Tiles[p.Tile].Owner != Bank {
		return
	}
	g.Tiles[p.Tile] = TileState{Owner: p.Winner}
	g.pendingAuctionWin = nil
	g.logf("auction_transfer", p.Winner, 0,
		"%s gehört nun %s, nachdem die Auktionsschuld beglichen wurde.",
		g.tile(p.Tile).Name, g.Players[p.Winner].Name)
}

// discardPendingAuctionWinFor drops a held-back win without transferring it.
func (g *Game) discardPendingAuctionWinFor(player int) {
	if g.pendingAuctionWin != nil && g.pendingAuctionWin.Winner == player {
		g.pendingAuctionWin = nil
	}
}

// Tick resolves expired auctions and runs at most one autopilot action. It
// reports whether anything changed and a broadcast is needed.
func (g *Game) Tick(now time.Time) bool {
	if g.Phase != PhasePlaying {
		return false
	}
	return g.tickAutopilot(now)
}
