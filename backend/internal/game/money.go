package game

import (
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// credit adds money paid by the bank.
func (g *Game) credit(player, amount int) {
	g.Players[player].Cash += amount
}

// settle transfers money from payer to the given creditors (Bank = -1). The
// full amount is always deducted, so a payer who cannot cover it ends up with a
// negative cash balance. That debt blocks further play until the player raises
// enough money by mortgaging or selling property (Spec.md §4.10). It returns
// the amount each creditor actually received.
func (g *Game) settle(payer int, claims map[int]int) map[int]int {
	received := map[int]int{}
	if payer < 0 || payer >= len(g.Players) || claims == nil {
		return received
	}
	total := 0
	for _, amount := range claims {
		if amount > 0 {
			total += amount
		}
	}
	if total == 0 {
		return received
	}
	p := g.Players[payer]
	p.Cash -= total
	for creditor, amount := range claims {
		if amount <= 0 {
			continue
		}
		if creditor == Bank {
			received[Bank] += amount
			continue
		}
		if g.active(creditor) {
			g.Players[creditor].Cash += amount
			received[creditor] += amount
		}
	}
	if p.Cash < 0 {
		g.logf("debt", payer, -p.Cash, "%s hat %d GE Schulden und muss Geld beschaffen.", p.Name, -p.Cash)
		if payer == g.Current && g.Phase == PhasePlaying {
			g.TurnPhase = TurnAwaitingDebt
		}
	}
	return received
}

// inDebt reports whether the player has an outstanding negative balance.
func (g *Game) inDebt(player int) bool {
	return player >= 0 && player < len(g.Players) && g.Players[player].Cash < 0
}

// settleDebtIfCleared ends the turn once the active player has raised enough
// money to reach a non-negative balance. A debt always consumes the turn, even
// after a double.
func (g *Game) settleDebtIfCleared() {
	if g.Phase != PhasePlaying || g.TurnPhase != TurnAwaitingDebt {
		return
	}
	p := g.Players[g.Current]
	if p.Cash < 0 {
		return
	}
	g.logf("debt_paid", g.Current, p.Cash, "%s hat die Schulden beglichen.", p.Name)
	g.extraTurn = false
	g.turnOver = true
	g.completeTurn()
}

// bankrupt removes the player from the game and returns their assets to the
// bank. It is used when a player leaves or gives up while in debt (Spec.md
// §4.1, §4.10); pending claims are only relevant for legacy callers.
func (g *Game) bankrupt(payer int, claims map[int]int) map[int]int {
	p := g.Players[payer]
	g.disableAutopilotSilently(payer)
	pending := p.Cash
	if pending < 0 {
		pending = 0
	}
	p.Cash = 0

	for idx := range g.Tiles {
		st := &g.Tiles[idx]
		if st.Owner != payer {
			continue
		}
		t := g.tile(idx)
		for st.Houses > 0 {
			pending += t.Build / 2
			if st.Houses == 5 {
				st.Houses = 4
			} else {
				st.Houses--
			}
		}
		if !st.Mortgaged {
			pending += t.Sell
		}
		*st = TileState{Owner: Bank}
	}

	// Get-out-of-jail cards go to the creditors, or back under their deck.
	creditors := make([]int, 0, len(claims))
	for i := range g.Players {
		if claims[i] > 0 && g.active(i) && i != payer {
			creditors = append(creditors, i)
		}
	}
	if len(creditors) == 0 {
		for _, id := range p.JailCards {
			if g.returnCardByID(id) {
				continue
			}
		}
	} else {
		for i, id := range p.JailCards {
			g.Players[creditors[i%len(creditors)]].JailCards = append(g.Players[creditors[i%len(creditors)]].JailCards, id)
		}
	}
	p.JailCards = nil

	for i := range g.Trades {
		t := g.Trades[i]
		if t.Status == TradeOpen && (t.From == payer || t.To == payer) {
			t.Status = TradeLapsed
		}
	}
	if g.Auction != nil && g.Auction.Bidder == payer {
		g.Auction.Bidder = Bank
		g.Auction.Bid = 0
		g.logf("auction", payer, 0, "%s scheidet aus der Auktion aus.", p.Name)
	}

	// Proportional split; the remainder is handed out 1 GE at a time in seat
	// order.
	totalClaims := 0
	for _, amount := range claims {
		if amount > 0 {
			totalClaims += amount
		}
	}
	received := map[int]int{}
	remaining := pending
	if len(creditors) == 1 {
		received[creditors[0]] = pending
		g.Players[creditors[0]].Cash += pending
		remaining = 0
	} else if len(creditors) > 1 && totalClaims > 0 {
		for _, c := range creditors {
			share := pending * claims[c] / totalClaims
			received[c] = share
			g.Players[c].Cash += share
			remaining -= share
		}
		for i := 0; remaining > 0; i++ {
			c := creditors[i%len(creditors)]
			received[c]++
			g.Players[c].Cash++
			remaining--
		}
	}

	g.logf("bankruptcy", payer, pending, "%s scheidet aus; die verbleibenden Werte gehen an die Bank.", p.Name)
	p.Bankrupt = true
	if payer == g.Current {
		g.turnOver = true
	}
	g.checkGameOver()
	return received
}

func (g *Game) returnCardByID(id string) bool {
	card, ok := g.Catalog.Card(id)
	if !ok {
		return false
	}
	deck := g.Decks[card.Deck]
	if deck == nil {
		return false
	}
	deck.Draw = append(deck.Draw, card.ID)
	return true
}

// eliminate removes a player from the running game, discards a pending purchase
// offer and ends the turn when the eliminated seat was active. It is the single
// elimination path for leaving, giving up in debt and forfeiting (Spec.md
// §4.10); all asset handling stays in bankrupt.
func (g *Game) eliminate(player int) {
	g.bankrupt(player, map[int]int{})
	if g.Phase != PhasePlaying || player != g.Current {
		return
	}
	g.Pending = nil
	g.turnOver = true
	g.completeTurn()
}

// Leave removes a player from a running game by declaring them bankrupt
// (Spec.md §4.1: leaving after the start means bankruptcy).
func (g *Game) Leave(player int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if !g.active(player) {
		return nil
	}
	g.logf("leave", player, 0, "%s verlässt das Spiel.", g.Players[player].Name)
	g.eliminate(player)
	return nil
}

// GiveUp lets the active player who cannot settle their debt forfeit the game.
// Their properties go back to the bank; the payment that caused the debt has
// already been credited to the creditor, so the bank absorbs the shortfall.
func (g *Game) GiveUp(player int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if player != g.Current || !g.active(player) {
		return g.fail("not_your_turn", "Du bist nicht am Zug.")
	}
	p := g.Players[player]
	if p.Cash >= 0 {
		return g.fail("no_debt", "Du hast keine Schulden und kannst nicht aufgeben.")
	}
	g.logf("give_up", player, 0, "%s gibt auf und scheidet aus dem Spiel aus.", p.Name)
	g.eliminate(player)
	return nil
}

// Forfeit lets the active player leave the game voluntarily in their own turn.
// The debt phase keeps its own GiveUp path (Spec.md §4.10).
func (g *Game) Forfeit(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	switch g.TurnPhase {
	case TurnAwaitingRoll, TurnAwaitingPurchase, TurnAwaitingJail:
	case TurnAwaitingDebt:
		return g.fail("wrong_phase", "In der Schuldenphase gibst du über den Schulden-Dialog auf.")
	default:
		return g.fail("wrong_phase", "Diese Aktion ist jetzt nicht möglich.")
	}
	g.logf("give_up", player, 0, "%s gibt auf und scheidet aus dem Spiel aus.", g.Players[player].Name)
	g.eliminate(player)
	return nil
}

func (g *Game) checkGameOver() {
	last := -1
	count := 0
	for i, p := range g.Players {
		if !p.Bankrupt {
			last = i
			count++
		}
	}
	if count > 1 || g.Phase == PhaseFinished {
		return
	}
	g.Phase = PhaseFinished
	g.TurnPhase = TurnFinished
	g.Pending = nil
	g.Auction = nil
	g.Winner = last
	for i := range g.Players {
		g.disableAutopilotSilently(i)
	}
	for i := range g.Trades {
		if g.Trades[i].Status == TradeOpen {
			g.Trades[i].Status = TradeLapsed
		}
	}
	if last >= 0 {
		g.logf("game_over", last, 0, "%s gewinnt das Spiel.", g.Players[last].Name)
	} else {
		g.logf("game_over", Bank, 0, "Das Spiel ist beendet.")
	}
}

// rentAmount returns the normal rent for a landing on idx using the current
// dice. It applies mortgage, jail and complete-group rules.
func (g *Game) rentAmount(idx int) int {
	t := g.tile(idx)
	st := g.Tiles[idx]
	if st.Owner == Bank || st.Owner < 0 || st.Owner >= len(g.Players) {
		return 0
	}
	owner := g.Players[st.Owner]
	if st.Mortgaged || owner.InJail || owner.Bankrupt {
		return 0
	}
	switch t.Type {
	case board.TypeStreet:
		if st.Houses > 0 {
			return t.Rents[st.Houses]
		}
		rent := t.Rents[0]
		if g.ownsAllStreets(st.Owner, t.Group) {
			rent *= 2
		}
		return rent
	case board.TypeTankstelle:
		n := g.countOwned(st.Owner, board.GroupTankstelle)
		if n < 1 {
			n = 1
		}
		if n > len(t.Rents) {
			n = len(t.Rents)
		}
		return t.Rents[n-1]
	case board.TypeUtility:
		mult := 4
		if g.countOwned(st.Owner, board.GroupVersorger) == 2 {
			mult = 10
		}
		return mult * (g.Dice[0] + g.Dice[1])
	}
	return 0
}

// chargeRent transfers rent from the tenant to the owner and applies the
// Subprime utility share.
func (g *Game) chargeRent(tenant, owner, idx, amount int) {
	if amount <= 0 {
		return
	}
	received := g.settle(tenant, map[int]int{owner: amount})
	paid := received[owner]
	if paid <= 0 {
		return
	}
	g.Players[owner].Stats.RentCollected += paid
	g.Players[tenant].Stats.RentPaid += paid
	g.logf("rent", tenant, paid, "%s zahlt %d GE Miete an %s.", g.Players[tenant].Name, paid, g.Players[owner].Name)
	g.applyUtilityShare(owner, idx, paid)
}

// applyUtilityShare pays 10 % of a rosa/orange/rot/gelb street rent to the
// matching utility owner, or to the bank when the utility is unowned.
func (g *Game) applyUtilityShare(owner, idx, rent int) {
	t := g.tile(idx)
	var utilityName string
	switch t.Group {
	case board.GroupRosa, board.GroupOrange:
		utilityName = "Elektrizitätswerk"
	case board.GroupRot, board.GroupGelb:
		utilityName = "Wasserwerk"
	default:
		return
	}
	if rent <= 0 {
		return
	}
	utility := g.Catalog.UtilityIndex(utilityName)
	if utility < 0 {
		return
	}
	utilityOwner := g.Tiles[utility].Owner
	if utilityOwner == owner {
		return
	}
	share := rent * 10 / 100
	if utilityOwner == Bank {
		share = (rent*10 + 99) / 100
		if share <= 0 {
			return
		}
		g.settle(owner, map[int]int{Bank: share})
		g.logf("utility_share", owner, share, "%s führt %d GE Nutzungsanteil an die Bank ab.", g.Players[owner].Name, share)
		return
	}
	if !g.active(utilityOwner) {
		return
	}
	if share <= 0 {
		return
	}
	g.settle(owner, map[int]int{utilityOwner: share})
	g.logf("utility_share", owner, share, "%s führt %d GE Nutzungsanteil an %s ab.", g.Players[owner].Name, share, g.Players[utilityOwner].Name)
}

func (g *Game) netWorth(idx int) int {
	p := g.Players[idx]
	total := p.Cash
	for i, st := range g.Tiles {
		if st.Owner != idx {
			continue
		}
		if st.Mortgaged {
			continue
		}
		total += g.tile(i).Price
	}
	return total
}

func (g *Game) payBank(player, amount int, kind, format string, args ...any) {
	if amount <= 0 {
		return
	}
	g.settle(player, map[int]int{Bank: amount})
	if kind != "" {
		g.logf(kind, player, amount, format, args...)
	}
}
