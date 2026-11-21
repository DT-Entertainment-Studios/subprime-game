package game

import (
	"fmt"
	"sort"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// credit adds money paid by the bank.
func (g *Game) credit(player, amount int) {
	g.Players[player].Cash += amount
}

// PaymentCause carries the stable cause code of a payment and an optional,
// already formatted creditor fragment ("an Berta", "für Rathaus"). The creditor
// fragment only ends up in the generated bankruptcy text, never in its own
// event field.
type PaymentCause struct {
	Code     string
	Creditor string
}

// BankruptcyDetails is the structured payload of an automatic bankruptcy. The
// three amounts are computed before the payer's assets are liquidated.
type BankruptcyDetails struct {
	Cause      string
	Creditor   string
	Debt       int
	Realizable int
	Shortfall  int
}

// firstCause returns the first supplied cause or the default "zahlung" when a
// caller did not provide one. This keeps two-argument settle callers working.
func firstCause(cause []PaymentCause) PaymentCause {
	if len(cause) > 0 && cause[0].Code != "" {
		return cause[0]
	}
	return PaymentCause{Code: "zahlung"}
}

// bankruptcyCauseLabel maps a cause code to its German label for the generated
// bankruptcy text. Unknown codes fall back to "Zahlung".
func bankruptcyCauseLabel(code string) string {
	switch code {
	case "miete":
		return "Miete"
	case "nutzungsanteil":
		return "Nutzungsanteil"
	case "auktion":
		return "Auktionskauf"
	case "steuer":
		return "Steuer"
	case "reparaturkosten":
		return "Reparaturkosten"
	case "kartenreparatur":
		return "Reparatur laut Karte"
	case "kartenzahlung":
		return "Kartenzahlung"
	case "zahlung_an_alle":
		return "Zahlung an alle"
	case "beitreibung":
		return "Beitreibung"
	case "zahlung":
		return "Zahlung"
	default:
		return "Zahlung"
	}
}

// settle transfers money from payer to the given creditors (Bank = -1). The
// full amount is always deducted, so a payer who cannot cover it ends up with a
// negative cash balance. Creditors are always paid in full; the bank absorbs
// any shortfall. A negative balance is handed to registerDebt, the single gate
// that either enters the debt phase or declares bankruptcy (Spec.md §4.10). It
// returns the amount each creditor actually received. cause is optional to keep
// legacy two-argument callers compiling.
func (g *Game) settle(payer int, claims map[int]int, cause ...PaymentCause) map[int]int {
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
		g.registerDebt(payer, firstCause(cause))
	}
	return received
}

// inDebt reports whether the player has an outstanding negative balance.
func (g *Game) inDebt(player int) bool {
	return player >= 0 && player < len(g.Players) && g.Players[player].Cash < 0
}

// settleDebtIfCleared ends the debt of the focused debtor once they have raised
// enough money to reach a non-negative balance. A debt of the active player
// consumes the turn (even after a double); a foreign debtor leaves the
// interrupted turn untouched. Either way the focus moves on via
// advanceDebtFocus, which is the only place that resumes the game.
func (g *Game) settleDebtIfCleared() {
	if g.Phase != PhasePlaying || g.TurnPhase != TurnAwaitingDebt || g.Debtor < 0 {
		return
	}
	p := g.Players[g.Debtor]
	if p.Cash < 0 {
		return
	}
	g.logf("debt_paid", g.Debtor, p.Cash, "%s hat die Schulden beglichen.", p.Name)
	g.settlePendingAuctionWin()
	if g.Debtor == g.Current {
		g.extraTurn = false
		g.turnOver = true
	}
	g.clearDebtFocus(g.Debtor)
	g.advanceDebtFocus()
}

// registerDebt is the single gate for becoming insolvent. It is called by
// settle and by the auction settlement and nowhere else. With no way to cover
// the balance it declares bankruptcy immediately (Spec.md §4.10 "D4");
// otherwise it parks the payer in the debt focus. It reports whether the payer
// was eliminated. The phase is not changed here: the calling turn pauses at
// its next checkpoint so synchronous card chains finish first.
func (g *Game) registerDebt(player int, cause PaymentCause) bool {
	if g.Phase != PhasePlaying || !g.inDebt(player) {
		return false
	}
	if liquidation := g.liquidationValue(player); liquidation < 0 {
		cash := g.Players[player].Cash
		details := BankruptcyDetails{
			Cause:      cause.Code,
			Creditor:   cause.Creditor,
			Debt:       -cash,
			Realizable: liquidation - cash,
			Shortfall:  -liquidation,
		}
		g.bankrupt(player, map[int]int{}, details)
		g.clearDebtFocus(player)
		return true
	}
	g.pauseForDebt(player)
	return false
}

// pauseForDebt registers payer as the open debtor or queues them behind the
// current focus. Debtors are unique; the queue is sorted by seat at the next
// checkpoint (A4).
func (g *Game) pauseForDebt(payer int) {
	if g.Debtor < 0 {
		g.Debtor = payer
		return
	}
	if g.Debtor == payer {
		return
	}
	for _, queued := range g.debtQueue {
		if queued == payer {
			return
		}
	}
	g.debtQueue = append(g.debtQueue, payer)
}

// debtCheckpoint pauses a turn at its end while a debt is open. It returns
// true when the caller must return instead of calling completeTurn. It is the
// only place that stops a debtor's autopilot because of debt.
func (g *Game) debtCheckpoint() bool {
	if g.Debtor < 0 {
		return false
	}
	sort.Ints(g.debtQueue)
	g.TurnPhase = TurnAwaitingDebt
	text := fmt.Sprintf("%s schaltet den Autopilot aus: Schulden müssen beglichen werden.",
		g.Players[g.Debtor].Name)
	g.stopAutopilot(g.Debtor, AutopilotStopDebt, text)
	return true
}

// clearDebtFocus removes player from the focus and the queue. It reports true
// only when the open focus was consumed, which tells eliminate whether it owns
// the resume. Removing a queued entry alone returns false.
func (g *Game) clearDebtFocus(player int) bool {
	ownedFocus := g.Debtor == player
	if ownedFocus {
		g.Debtor = -1
	}
	remaining := g.debtQueue[:0]
	for _, queued := range g.debtQueue {
		if queued != player {
			remaining = append(remaining, queued)
		}
	}
	g.debtQueue = remaining
	return ownedFocus
}

// advanceDebtFocus is the only resume path after a debt. Callers must have
// cleared the settled/bankrupt focus first (Debtor == -1). It skips queued
// seats that are no longer active or no longer in debt, activates the next
// debtor, or, when nothing remains, clears the focus and runs exactly one
// completeTurn. completeTurn is never entered while a debt is still open.
func (g *Game) advanceDebtFocus() {
	for g.Debtor < 0 && len(g.debtQueue) > 0 {
		next := g.debtQueue[0]
		g.debtQueue = g.debtQueue[1:]
		if g.active(next) && g.inDebt(next) {
			g.Debtor = next
			g.TurnPhase = TurnAwaitingDebt
			return
		}
	}
	g.Debtor = -1
	g.debtQueue = nil
	g.completeTurn()
}

// bankrupt removes the player from the game and returns their assets to the
// bank. It is used when a player leaves or gives up while in debt (Spec.md
// §4.1, §4.10); pending claims are only relevant for legacy callers.
func (g *Game) bankrupt(payer int, claims map[int]int, details ...BankruptcyDetails) map[int]int {
	p := g.Players[payer]
	g.discardPendingAuctionWinFor(payer)
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

	if len(details) > 0 {
		g.logBankruptcy(payer, details[0])
	} else {
		g.logf("bankruptcy", payer, pending,
			"%s scheidet aus; die verbleibenden Werte gehen an die Bank.", p.Name)
	}
	p.EliminatedAt = g.Turn
	p.Bankrupt = true
	if payer == g.Current {
		g.turnOver = true
	}
	g.checkGameOver()
	return received
}

// logBankruptcy appends the single authoritative event for an automatic
// bankruptcy. It follows the logCard pattern and leaves reason untouched.
func (g *Game) logBankruptcy(player int, d BankruptcyDetails) {
	p := g.Players[player]
	label := bankruptcyCauseLabel(d.Cause)
	if d.Creditor != "" {
		label += " " + d.Creditor
	}
	text := fmt.Sprintf("%s ist pleite. %s: %d GE. Realisierbar: %d GE. "+
		"Fehlbetrag: %d GE. Seine Felder gehen an die Bank.",
		p.Name, label, d.Debt, d.Realizable, d.Shortfall)
	g.eventSeq++
	g.Events = append(g.Events, Event{
		Seq:        g.eventSeq,
		Type:       "bankruptcy",
		Player:     player,
		Cause:      d.Cause,
		Debt:       d.Debt,
		Realizable: d.Realizable,
		Shortfall:  d.Shortfall,
		Text:       text,
		At:         g.clock(),
	})
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

// eliminate removes a player from the running game, discards a pending
// purchase offer and owns the resume when the eliminated seat held the debt
// focus. It is the single elimination path for leaving, giving up in debt and
// forfeiting (Spec.md §4.10); all asset handling stays in bankrupt. The focus
// is cleared before bankrupt so a finished game cannot leave a dangling debtor.
// completeTurn is never called while another seat still owes money.
func (g *Game) eliminate(player int) {
	ownedFocus := g.clearDebtFocus(player)
	g.bankrupt(player, map[int]int{})
	if g.Phase != PhasePlaying {
		return
	}
	if ownedFocus {
		g.advanceDebtFocus()
		return
	}
	if player != g.Current {
		return
	}
	g.Pending = nil
	g.turnOver = true
	if g.Debtor >= 0 {
		// Another seat still owes money. Keep the waiting phase and leave the
		// resume to that debtor's advanceDebtFocus; turning now would drop it.
		return
	}
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

// GiveUp lets the focused debtor forfeit the game, even when it is not their
// turn. Their properties go back to the bank; the payment that caused the debt
// has already been credited to the creditor, so the bank absorbs the shortfall.
// Focus cleanup and the resume belong to eliminate, not here.
func (g *Game) GiveUp(player int) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	if player != g.Debtor || !g.active(player) {
		return g.fail("not_your_turn", "Du bist nicht der Schuldner.")
	}
	p := g.Players[player]
	if !g.inDebt(player) {
		return g.fail("no_debt", "Du hast keine Schulden und kannst nicht aufgeben.")
	}
	g.logf("give_up", player, 0, "%s gibt auf und scheidet aus dem Spiel aus.", p.Name)
	g.eliminate(player)
	return nil
}

// Forfeit lets the active player leave the game voluntarily in their own turn.
// It is never available during the debt phase, which keeps its own GiveUp path
// (Spec.md §4.10).
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
	g.Debtor = -1
	g.debtQueue = nil
	g.pendingAuctionWin = nil
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
	received := g.settle(tenant, map[int]int{owner: amount},
		PaymentCause{Code: "miete", Creditor: "an " + g.Players[owner].Name})
	paid := received[owner]
	if paid <= 0 {
		return
	}
	g.Players[owner].Stats.RentCollected += paid
	g.Players[tenant].Stats.RentPaid += paid
	if g.Players[tenant].Stats.RentPaidTo == nil {
		g.Players[tenant].Stats.RentPaidTo = map[int]int{}
	}
	g.Players[tenant].Stats.RentPaidTo[owner] += paid
	g.logf("rent", tenant, paid, "%s zahlt %d GE Miete an %s.", g.Players[tenant].Name, paid, g.Players[owner].Name)
	g.applyUtilityShare(owner, idx, paid)
}

// applyUtilityShare pays 10 % of a rosa/orange/rot/gelb street rent to the
// matching utility owner; no share when the utility is unowned, mortgaged or
// owned by the street owner / an inmate.
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
	if utilityOwner == Bank {
		return
	}
	if g.Tiles[utility].Mortgaged {
		return
	}
	if utilityOwner >= 0 && utilityOwner < len(g.Players) && g.Players[utilityOwner].InJail {
		return
	}
	if utilityOwner == owner {
		return
	}
	share := rent * 10 / 100
	if !g.active(utilityOwner) {
		return
	}
	if share <= 0 {
		return
	}
	g.settle(owner, map[int]int{utilityOwner: share},
		PaymentCause{Code: "nutzungsanteil", Creditor: "an " + g.Players[utilityOwner].Name})
	g.logf("utility_share", owner, share, "%s führt %d GE Nutzungsanteil an %s ab.", g.Players[owner].Name, share, g.Players[utilityOwner].Name)
}

// liquidationValue is the maximum a player can raise without outside help: their
// cash plus every building refund (Build/2 each, a hotel counting as five)
// plus the bank sale value of every unmortgaged property. Mortgaged properties
// count zero. It deliberately differs from netWorth, which uses full prices.
func (g *Game) liquidationValue(player int) int {
	if player < 0 || player >= len(g.Players) {
		return 0
	}
	total := g.Players[player].Cash
	for idx, st := range g.Tiles {
		if st.Owner != player {
			continue
		}
		t := g.tile(idx)
		if st.Houses > 0 {
			total += st.Houses * (t.Build / 2)
		}
		if !st.Mortgaged {
			total += t.Sell
		}
	}
	return total
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

func (g *Game) payBank(player, amount int, cause PaymentCause, kind, format string, args ...any) {
	if amount <= 0 {
		return
	}
	g.settle(player, map[int]int{Bank: amount}, cause)
	if kind != "" {
		g.logf(kind, player, amount, format, args...)
	}
}
