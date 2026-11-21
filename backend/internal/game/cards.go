package game

import (
	"fmt"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

// drawCard removes the top card of a deck. An empty draw pile is refilled from
// the cards that are not currently held (defensive; with bottom-of-pile
// recycling the pile never empties in practice).
func (g *Game) drawCard(deck board.Deck) board.Card {
	d := g.Decks[deck]
	if d == nil || len(d.Draw) == 0 {
		return board.Card{}
	}
	id := d.Draw[0]
	d.Draw = d.Draw[1:]
	card, ok := g.Catalog.Card(id)
	if !ok {
		return board.Card{}
	}
	return card
}

// drawAndApply draws and executes a card, then returns it under the pile unless
// the player keeps it.
func (g *Game) drawAndApply(player int, deck board.Deck, depth int) {
	if g.Phase != PhasePlaying || depth > 20 {
		return
	}
	card := g.drawCard(deck)
	if card.ID == "" {
		return
	}
	g.logCard(player, card)
	keep := false
	switch card.Effect {
	case board.EffectAdvanceTo:
		if card.Target != nil {
			pos := g.Players[player].Position
			steps := (*card.Target - pos + 40) % 40
			if steps > 0 {
				g.moveForward(player, steps, card.LosSalary)
				g.resolveLanding(player, g.Players[player].Position, depth)
			}
		}
	case board.EffectMoveBy:
		if card.Target != nil {
			g.moveBackward(player, -*card.Target)
			g.resolveLanding(player, g.Players[player].Position, depth)
		}
	case board.EffectGoToJail:
		g.sendToJail(player)
		g.turnOver = true
	case board.EffectGetOutOfJail:
		g.Players[player].JailCards = append(g.Players[player].JailCards, card.ID)
		keep = true
		g.logf("jail_card", player, 0, "%s behält den Werkstattgutschein.", g.Players[player].Name)
	case board.EffectBankPayout:
		g.credit(player, card.Amount)
		g.logf("payout", player, card.Amount, "Die Bank zahlt %s %d GE.", g.Players[player].Name, card.Amount)
	case board.EffectBankPayment:
		g.payBank(player, card.Amount, PaymentCause{Code: "kartenzahlung"}, "payment",
			"%s zahlt %d GE an die Bank.", g.Players[player].Name, card.Amount)
	case board.EffectPayEachPlayer:
		claims := map[int]int{}
		for i := range g.Players {
			if i != player && g.active(i) {
				claims[i] = card.Amount
			}
		}
		g.logf("payment", player, card.Amount*len(claims), "%s zahlt jedem Mitspieler %d GE.", g.Players[player].Name, card.Amount)
		g.settle(player, claims, PaymentCause{Code: "zahlung_an_alle"})
	case board.EffectCollectFromEachPlayer:
		for i := range g.Players {
			if i == player || !g.active(i) || g.Phase != PhasePlaying {
				continue
			}
			g.settle(i, map[int]int{player: card.Amount},
				PaymentCause{Code: "beitreibung", Creditor: "an " + g.Players[player].Name})
			g.logf("payout", i, card.Amount, "%s schenkt %s %d GE.", g.Players[i].Name, g.Players[player].Name, card.Amount)
		}
	case board.EffectPayPerHouse:
		houses, hotels := g.buildingCounts(player)
		amount := houses*card.Amount + hotels*card.PerHotel
		if amount > 0 {
			g.payBank(player, amount, PaymentCause{Code: "kartenreparatur"}, "payment",
				"%s zahlt %d GE für %d Häuser und %d Hotels.",
				g.Players[player].Name, amount, houses, hotels)
		}
	case board.EffectNextTankstelle:
		g.advanceToNext(player, board.TypeTankstelle, depth)
	case board.EffectNextUtility:
		g.advanceToNext(player, board.TypeUtility, depth)
	}
	if !keep {
		g.returnCardByID(card.ID)
	}
}

// logCard records a card draw with the deck it came from, so the client can
// show the drawn card as a card popup instead of only as a log line.
func (g *Game) logCard(player int, card board.Card) {
	g.eventSeq++
	g.Events = append(g.Events, Event{
		Seq:    g.eventSeq,
		Type:   "card",
		Player: player,
		Text:   fmt.Sprintf("%s zieht: %s", g.Players[player].Name, card.Text),
		Deck:   card.Deck,
		At:     g.clock(),
	})
}

func (g *Game) buildingCounts(player int) (houses, hotels int) {
	for _, st := range g.Tiles {
		if st.Owner != player {
			continue
		}
		if st.Houses == 5 {
			hotels++
		} else {
			houses += st.Houses
		}
	}
	return houses, hotels
}

// advanceToNext implements the two next-tankstelle/next-utility cards with their
// own rent rules (Spec.md §3.2).
func (g *Game) advanceToNext(player int, tileType board.TileType, depth int) {
	from := g.Players[player].Position
	steps := 0
	target := -1
	for i := 1; i <= 40; i++ {
		idx := (from + i) % 40
		if g.tile(idx).Type == tileType {
			target = idx
			steps = i
			break
		}
	}
	if target < 0 {
		return
	}
	g.moveForward(player, steps, true)
	idx := target
	if g.Tiles[idx].Owner == Bank {
		g.offerPurchase(player, idx)
		return
	}
	if g.Tiles[idx].Owner == player {
		g.logf("land", player, 0, "%s landet auf dem eigenen Feld %s.", g.Players[player].Name, g.tile(idx).Name)
		return
	}
	if g.Tiles[idx].Mortgaged {
		g.logf("land", player, 0, "%s ist hypothekiert; es ist keine Miete fällig.", g.tile(idx).Name)
		return
	}
	owner := g.Tiles[idx].Owner
	if g.Players[owner].InJail {
		g.logf("land", player, 0, "%s ist in der Werkstatt und zieht keine Miete ein.", g.Players[owner].Name)
		return
	}
	if tileType == board.TypeTankstelle {
		normal := g.rentAmount(idx)
		if normal > 0 {
			g.chargeRent(player, owner, idx, normal*2)
			g.logf("card_rent", player, normal*2, "Die Tankstellenkarte verdoppelt die Miete.")
		}
		return
	}
	d1, d2 := g.rollDice()
	g.Dice = [2]int{d1, d2}
	total := d1 + d2
	amount := total * 10
	g.logf("roll", player, total, "%s würfelt für das Versorgungswerk %d und %d.", g.Players[player].Name, d1, d2)
	g.chargeRent(player, owner, idx, amount)
}
