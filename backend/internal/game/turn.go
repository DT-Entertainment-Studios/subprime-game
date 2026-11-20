package game

import (
	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// Jail options accepted by JailOption.
const (
	JailOptionRoll = "roll"
	JailOptionPay  = "pay"
	JailOptionCard = "card"
)

// Roll performs the active player's dice roll (Spec.md §4.2).
func (g *Game) Roll(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	if g.inDebt(player) {
		return g.fail("in_debt", "Begleiche zuerst deine Schulden.")
	}
	if g.TurnPhase == TurnAwaitingJail {
		return g.fail("wrong_phase", "Wähle zuerst eine Werkstatt-Option.")
	}
	if err := g.requireTurnPhase(TurnAwaitingRoll); err != nil {
		return err
	}
	g.lapseTrades(player)

	d1, d2 := g.rollDice()
	g.Dice = [2]int{d1, d2}
	total := d1 + d2
	g.logf("roll", player, total, "%s würfelt %d und %d.", g.Players[player].Name, d1, d2)

	if d1 == d2 {
		g.Doubles++
	} else {
		g.Doubles = 0
	}
	if g.Doubles >= 3 {
		g.logf("jail", player, 0, "Dreimal hintereinander ein Pasch: %s verunfallt und muss in die Werkstatt.", g.Players[player].Name)
		g.sendToJail(player)
		g.extraTurn = false
		g.turnOver = true
		g.completeTurn()
		return nil
	}

	g.extraTurn = d1 == d2
	g.turnOver = false
	g.moveForward(player, total, true)
	g.resolveLanding(player, g.Players[player].Position, 0)
	if g.debtCheckpoint() {
		return nil
	}
	g.completeTurn()
	return nil
}

// JailOption resolves the active player's jail decision.
func (g *Game) JailOption(player int, option string) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	if err := g.requireTurnPhase(TurnAwaitingJail); err != nil {
		return err
	}
	p := g.Players[player]
	switch option {
	case JailOptionRoll:
		g.attemptJailRoll(player)
	case JailOptionPay:
		if p.Cash < g.Rules.JailFine {
			return g.fail("insufficient_cash", "Du kannst die Reparaturkosten nicht bezahlen.")
		}
		g.payBank(player, g.Rules.JailFine, PaymentCause{Code: "reparaturkosten"}, "jail_fine",
			"%s zahlt %d GE Reparaturkosten.", p.Name, g.Rules.JailFine)
		if g.Phase == PhaseFinished || p.Bankrupt || p.Cash < 0 {
			if g.debtCheckpoint() {
				return nil
			}
			g.completeTurn()
			return nil
		}
		g.leaveJail(player)
		g.logf("jail_exit", player, 0, "%s verlässt die Werkstatt gegen Bezahlung.", p.Name)
		g.normalRollAfterJail(player)
	case JailOptionCard:
		if len(p.JailCards) == 0 {
			return g.fail("no_card", "Du hast keine Pannenhilfe.")
		}
		id := p.JailCards[0]
		p.JailCards = p.JailCards[1:]
		g.returnCardByID(id)
		g.leaveJail(player)
		g.logf("jail_exit", player, 0, "%s nutzt eine Pannenhilfe.", p.Name)
		g.normalRollAfterJail(player)
	default:
		return g.fail("invalid_option", "Unbekannte Werkstatt-Option.")
	}
	return nil
}

func (g *Game) attemptJailRoll(player int) {
	p := g.Players[player]
	g.lapseTrades(player)
	d1, d2 := g.rollDice()
	g.Dice = [2]int{d1, d2}
	total := d1 + d2
	g.logf("roll", player, total, "%s würfelt in der Werkstatt %d und %d.", p.Name, d1, d2)
	if d1 == d2 {
		g.leaveJail(player)
		g.logf("jail_exit", player, 0, "%s springt mit einem Pasch wieder an.", p.Name)
		g.extraTurn = false
		g.turnOver = false
		g.moveForward(player, total, true)
		g.resolveLanding(player, p.Position, 0)
		if g.debtCheckpoint() {
			return
		}
		g.completeTurn()
		return
	}

	p.JailAttempts++
	if p.JailAttempts >= 3 {
		g.logf("jail", player, 0, "Dritter Fehlversuch: %s muss die Reparaturkosten von %d GE zahlen.", p.Name, g.Rules.JailFine)
		g.payBank(player, g.Rules.JailFine, PaymentCause{Code: "reparaturkosten"}, "jail_fine",
			"%s zahlt %d GE Reparaturkosten.", p.Name, g.Rules.JailFine)
		if g.Phase == PhaseFinished || p.Bankrupt {
			if g.debtCheckpoint() {
				return
			}
			g.completeTurn()
			return
		}
		g.leaveJail(player)
		g.logf("jail_exit", player, 0, "%s kommt nach der Reparatur frei.", p.Name)
		if p.Cash < 0 {
			if g.debtCheckpoint() {
				return
			}
			g.completeTurn()
			return
		}
		g.extraTurn = false
		g.turnOver = false
		g.moveForward(player, total, true)
		g.resolveLanding(player, p.Position, 0)
		if g.debtCheckpoint() {
			return
		}
		g.completeTurn()
		return
	}

	g.logf("jail", player, 0, "%s bleibt in der Werkstatt (Versuch %d von 3).", p.Name, p.JailAttempts)
	g.extraTurn = false
	g.turnOver = true
	g.completeTurn()
}

func (g *Game) normalRollAfterJail(player int) {
	g.lapseTrades(player)
	d1, d2 := g.rollDice()
	g.Dice = [2]int{d1, d2}
	total := d1 + d2
	g.logf("roll", player, total, "%s würfelt %d und %d.", g.Players[player].Name, d1, d2)
	g.Doubles = 0
	if d1 == d2 {
		g.Doubles = 1
	}
	g.extraTurn = d1 == d2
	g.turnOver = false
	g.moveForward(player, total, true)
	g.resolveLanding(player, g.Players[player].Position, 0)
	if g.debtCheckpoint() {
		return
	}
	g.completeTurn()
}

func (g *Game) leaveJail(player int) {
	g.Players[player].InJail = false
	g.Players[player].JailAttempts = 0
}

func (g *Game) sendToJail(player int) {
	p := g.Players[player]
	p.Position = 10
	p.InJail = true
	p.JailAttempts = 0
	g.logf("jail", player, 0, "%s muss in die Werkstatt.", p.Name)
}

func (g *Game) rollDice() (int, int) {
	if g.dice != nil {
		return g.dice()
	}
	return g.rand.Intn(6) + 1, g.rand.Intn(6) + 1
}

// moveForward moves the player clockwise, paying the Los salary when Los is
// passed or landed on.
func (g *Game) moveForward(player, steps int, salary bool) {
	if steps <= 0 {
		return
	}
	p := g.Players[player]
	old := p.Position
	target := old + steps
	if salary && target >= len(g.Catalog.Tiles) {
		g.credit(player, g.Rules.LosSalary)
		g.logf("salary", player, g.Rules.LosSalary, "%s zieht %d GE Gehalt für Los ein.", p.Name, g.Rules.LosSalary)
	}
	p.Position = target % len(g.Catalog.Tiles)
	g.logf("move", player, 0, "%s zieht von Feld %d auf Feld %d.", p.Name, old, p.Position)
}

func (g *Game) moveBackward(player, steps int) {
	p := g.Players[player]
	old := p.Position
	p.Position = (p.Position - steps + 40) % 40
	g.logf("move_back", player, 0, "%s zieht von Feld %d auf Feld %d zurück.", p.Name, old, p.Position)
}

// resolveLanding resolves whatever waits on idx. depth bounds card chains.
func (g *Game) resolveLanding(player, idx, depth int) {
	if g.Phase != PhasePlaying || g.Players[player].Bankrupt || depth > 20 {
		return
	}
	t := g.tile(idx)
	switch t.Type {
	case board.TypeStreet, board.TypeTankstelle, board.TypeUtility:
		g.landProperty(player, idx)
	case board.TypeTax:
		g.landTax(player, t)
	case board.TypeChance:
		g.drawAndApply(player, board.DeckChance, depth+1)
	case board.TypeChest:
		g.drawAndApply(player, board.DeckChest, depth+1)
	case board.TypeGotoJail:
		g.sendToJail(player)
		g.turnOver = true
	case board.TypeGo, board.TypeJail, board.TypeParking:
		// Nothing happens; Los salary is handled by the movement.
	}
}

// landTax charges a tax tile: either its fixed amount or a percentage of the
// player's current cash (rounded down to whole GE).
func (g *Game) landTax(player int, t board.Tile) {
	name := g.Players[player].Name
	if t.TaxPercent > 0 {
		amount := g.Players[player].Cash * t.TaxPercent / 100
		g.payBank(player, amount, PaymentCause{Code: "steuer"}, "tax", "%s zahlt %d GE %s (%d %% des Bargelds).", name, amount, t.Name, t.TaxPercent)
		return
	}
	g.payBank(player, t.Tax, PaymentCause{Code: "steuer"}, "tax", "%s zahlt %d GE %s.", name, t.Tax, t.Name)
}

// landProperty resolves a landing on an ownable tile with the normal rent
// rules.
func (g *Game) landProperty(player, idx int) {
	st := g.Tiles[idx]
	if st.Owner == Bank {
		g.offerPurchase(player, idx)
		return
	}
	if st.Owner == player {
		g.logf("land", player, 0, "%s landet auf dem eigenen Feld %s.", g.Players[player].Name, g.tile(idx).Name)
		return
	}
	if st.Mortgaged {
		g.logf("land", player, 0, "%s ist hypothekiert; es ist keine Miete fällig.", g.tile(idx).Name)
		return
	}
	owner := g.Players[st.Owner]
	if owner.InJail {
		g.logf("land", player, 0, "%s ist in der Werkstatt und zieht keine Miete ein.", owner.Name)
		return
	}
	amount := g.rentAmount(idx)
	if amount <= 0 {
		return
	}
	g.chargeRent(player, st.Owner, idx, amount)
}

// offerPurchase offers an unowned tile. A player who cannot cover the price
// right now still gets the offer so they can raise money by mortgaging or
// selling property; the auction only starts when the player declines
// (Spec.md §4.4).
func (g *Game) offerPurchase(player, idx int) {
	price, doubled := g.priceFor(player, idx)
	if g.Players[player].Cash < price {
		g.logf("offer", player, price, "%s kann %s aktuell nicht bezahlen und kann Geld durch Hypotheken oder Verkäufe beschaffen.", g.Players[player].Name, g.tile(idx).Name)
	}
	g.Pending = &PendingPurchase{
		Tile:    idx,
		Price:   price,
		Doubled: doubled,
	}
	g.TurnPhase = TurnAwaitingPurchase
	g.logf("offer", player, price, "%s kann %s für %d GE kaufen.", g.Players[player].Name, g.tile(idx).Name, price)
}

// Buy accepts the pending purchase.
func (g *Game) Buy(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	if err := g.requireTurnPhase(TurnAwaitingPurchase); err != nil {
		return err
	}
	pending := g.Pending
	if pending == nil || g.Tiles[pending.Tile].Owner != Bank {
		g.Pending = nil
		g.completeTurn()
		return g.fail("invalid_state", "Kein Kaufangebot vorhanden.")
	}
	p := g.Players[player]
	if p.Cash < pending.Price {
		return g.fail("insufficient_cash", "Du kannst dieses Feld nicht bezahlen.")
	}
	p.Cash -= pending.Price
	g.Tiles[pending.Tile] = TileState{Owner: player}
	p.Stats.PropertiesBought++
	g.logf("buy", player, pending.Price, "%s kauft %s für %d GE.", p.Name, g.tile(pending.Tile).Name, pending.Price)
	if pending.Doubled {
		g.logf("double_price", player, pending.Price, "Doppelter Preis: %s vervollständigt die Farbgruppe.", p.Name)
	}
	g.Pending = nil
	g.completeTurn()
	return nil
}

// Decline declines the purchase and starts the auction.
func (g *Game) Decline(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	if err := g.requireTurnPhase(TurnAwaitingPurchase); err != nil {
		return err
	}
	if g.Pending == nil {
		return g.fail("invalid_state", "Kein Kaufangebot vorhanden.")
	}
	g.logf("decline", player, 0, "%s lehnt den Kauf von %s ab.", g.Players[player].Name, g.tile(g.Pending.Tile).Name)
	g.startAuction(g.Pending.Tile)
	return nil
}

// EndTurn is kept as an explicit intent for clients. Turns advance
// automatically; this only succeeds when there is nothing left to decide.
func (g *Game) EndTurn(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	if g.Debtor >= 0 || g.inDebt(player) {
		return g.fail("in_debt", "Begleiche zuerst deine Schulden.")
	}
	if g.Pending != nil || g.Auction != nil {
		return g.fail("wrong_phase", "Es steht noch eine Entscheidung aus.")
	}
	g.completeTurn()
	return nil
}

// completeTurn advances the turn or grants the extra roll for a double. It is
// only reached with no open debt focus: debts pause at the preceding checkpoint
// and resume through advanceDebtFocus, so an interrupted turn is never lost.
func (g *Game) completeTurn() {
	if g.Phase != PhasePlaying {
		g.TurnPhase = TurnFinished
		return
	}
	if g.inDebt(g.Current) {
		if g.Debtor < 0 {
			g.Debtor = g.Current
		}
		g.TurnPhase = TurnAwaitingDebt
		return
	}
	if g.Pending != nil || g.Auction != nil {
		return
	}
	if g.turnOver || !g.extraTurn {
		g.advanceTurn()
		return
	}
	g.turnOver = false
	g.extraTurn = false
	g.TurnPhase = TurnAwaitingRoll
	g.logf("turn", g.Current, 0, "%s darf wegen eines Paschs erneut würfeln.", g.Players[g.Current].Name)
	g.armAutopilot(g.Current)
}

func (g *Game) advanceTurn() {
	if g.Phase != PhasePlaying {
		g.TurnPhase = TurnFinished
		return
	}
	g.Pending = nil
	g.Auction = nil
	g.extraTurn = false
	g.turnOver = false
	g.Doubles = 0
	start := g.Current
	for i := 1; i <= len(g.Players); i++ {
		next := (start + i) % len(g.Players)
		if !g.Players[next].Bankrupt {
			g.Current = next
			break
		}
	}
	if g.inDebt(g.Current) {
		g.Debtor = g.Current
		g.TurnPhase = TurnAwaitingDebt
	} else {
		// A debt focus must never linger on a seat that is no longer active.
		g.Debtor = -1
		if g.Players[g.Current].InJail {
			g.TurnPhase = TurnAwaitingJail
		} else {
			g.TurnPhase = TurnAwaitingRoll
		}
	}
	g.logf("turn", g.Current, 0, "%s ist am Zug.", g.Players[g.Current].Name)
	g.armAutopilot(g.Current)
}

func (g *Game) lapseTrades(player int) {
	for i := range g.Trades {
		t := g.Trades[i]
		if t.Status == TradeOpen && (t.From == player || t.To == player) {
			t.Status = TradeLapsed
			g.logf("trade_lapsed", player, 0, "Das Handelsangebot mit %s ist mit dem Würfeln verfallen.", g.Players[t.From].Name)
		}
	}
}
