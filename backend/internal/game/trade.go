package game

import "fmt"

// TradeInput is a trade proposal submitted by a client. CounterTo optionally
// references an open offer this proposal supersedes (a counter-proposal).
type TradeInput struct {
	To           int    `json:"to"`
	OfferTiles   []int  `json:"offerTiles"`
	RequestTiles []int  `json:"requestTiles"`
	OfferCash    int    `json:"offerCash"`
	RequestCash  int    `json:"requestCash"`
	OfferCards   int    `json:"offerCards"`
	RequestCards int    `json:"requestCards"`
	CounterTo    string `json:"counterTo,omitempty"`
}

// ProposeTrade validates and stores a trade offer (Spec.md §4.8).
func (g *Game) ProposeTrade(from int, in TradeInput) (string, error) {
	if g.Phase != PhasePlaying {
		return "", g.fail("game_over", "Das Spiel ist beendet.")
	}
	if !g.active(from) || !g.active(in.To) || from == in.To {
		return "", g.fail("not_permitted", "Dieser Handel ist nicht möglich.")
	}
	if (from == g.Current || in.To == g.Current) && !g.activeTradeWindow() {
		return "", g.fail("wrong_phase", "Der aktive Spieler kann erst vor dem Würfeln handeln.")
	}
	if in.OfferCash < 0 || in.RequestCash < 0 || in.OfferCards < 0 || in.RequestCards < 0 {
		return "", g.fail("invalid_trade", "Negative Beträge sind nicht erlaubt.")
	}
	if err := g.validateTradeSide(from, in.OfferTiles, in.OfferCash, in.OfferCards); err != nil {
		return "", err
	}
	if err := g.validateTradeSide(in.To, in.RequestTiles, in.RequestCash, in.RequestCards); err != nil {
		return "", err
	}
	if len(in.OfferTiles)+len(in.RequestTiles)+in.OfferCards+in.RequestCards == 0 && in.OfferCash == 0 && in.RequestCash == 0 {
		return "", g.fail("invalid_trade", "Der Handel ist leer.")
	}
	var superseded *TradeOffer
	if in.CounterTo != "" {
		// A counter-proposal supersedes one specific open offer between the
		// same two players; every other open offer between them is rejected as
		// before. Who sends the counter does not matter — the offer is replaced
		// either way and both sides see the replacement.
		superseded = g.openTradeByID(in.CounterTo)
		if superseded == nil || !g.between(superseded, from, in.To) {
			return "", g.fail("trade_not_found", "Das zu konternde Angebot ist nicht mehr offen.")
		}
	} else {
		for _, t := range g.Trades {
			if t.Status == TradeOpen && ((t.From == from && t.To == in.To) || (t.From == in.To && t.To == from)) {
				return "", g.fail("trade_pending", "Zwischen euch liegt bereits ein offenes Angebot.")
			}
		}
	}
	id := fmt.Sprintf("trade-%d", len(g.Trades)+1)
	offer := &TradeOffer{
		ID:           id,
		From:         from,
		To:           in.To,
		OfferTiles:   append([]int(nil), in.OfferTiles...),
		RequestTiles: append([]int(nil), in.RequestTiles...),
		OfferCash:    in.OfferCash,
		RequestCash:  in.RequestCash,
		OfferCards:   in.OfferCards,
		RequestCards: in.RequestCards,
		Status:       TradeOpen,
		CreatedAt:    g.clock(),
	}
	g.Trades = append(g.Trades, offer)
	if superseded != nil {
		superseded.Status = TradeCountered
		other := superseded.From
		if other == from {
			other = superseded.To
		}
		g.logf("trade", from, 0, "%s kontert das Angebot von %s mit einem Gegengebot.", g.Players[from].Name, g.Players[other].Name)
	}
	g.logf("trade", from, 0, "%s schlägt %s einen Handel vor.", g.Players[from].Name, g.Players[in.To].Name)
	stopText := fmt.Sprintf(
		"%s schaltet den Autopilot aus: eingegangenes Tauschangebot.",
		g.Players[in.To].Name,
	)
	g.stopAutopilot(in.To, AutopilotStopTradeOffer, stopText)
	return id, nil
}

// activeTradeWindow reports whether the active player may trade right now:
// the pre-turn window before rolling, or the workshop decision that precedes
// the jail roll (Spec.md §4.2, §4.8).
func (g *Game) activeTradeWindow() bool {
	return g.TurnPhase == TurnAwaitingRoll || g.TurnPhase == TurnAwaitingJail
}

// openTradeByID returns the still-open offer with the given id.
func (g *Game) openTradeByID(id string) *TradeOffer {
	for _, t := range g.Trades {
		if t.ID == id && t.Status == TradeOpen {
			return t
		}
	}
	return nil
}

// between reports whether the offer runs between exactly these two players in
// either direction.
func (g *Game) between(offer *TradeOffer, a, b int) bool {
	return (offer.From == a && offer.To == b) || (offer.From == b && offer.To == a)
}

func (g *Game) validateTradeSide(player int, tiles []int, cash, cards int) error {
	seen := map[int]bool{}
	for _, idx := range tiles {
		if idx < 0 || idx >= len(g.Tiles) {
			return g.fail("invalid_trade", "Unbekanntes Feld.")
		}
		if seen[idx] {
			return g.fail("invalid_trade", "Ein Feld darf nur einmal im Handel stehen.")
		}
		seen[idx] = true
		if g.Tiles[idx].Owner != player {
			return g.fail("invalid_trade", "Nicht alle Felder gehören dem Anbieter.")
		}
		if g.Tiles[idx].Houses > 0 {
			return g.fail("invalid_trade", "Bebaute Felder können nicht gehandelt werden.")
		}
		if !g.tile(idx).Buyable() {
			return g.fail("invalid_trade", "Dieses Feld ist nicht handelbar.")
		}
	}
	if cards > len(g.Players[player].JailCards) {
		return g.fail("invalid_trade", "So viele Werkstattgutscheine besitzt der Spieler nicht.")
	}
	if cash > 0 && cash > g.Players[player].Cash {
		return g.fail("insufficient_cash", "So viel Bargeld besitzt der Spieler nicht.")
	}
	if cards < 0 {
		return g.fail("invalid_trade", "Ungültige Kartenanzahl.")
	}
	return nil
}

// RespondTrade accepts or declines an open offer.
func (g *Game) RespondTrade(player int, id string, accept bool) error {
	if g.Phase != PhasePlaying {
		return g.fail("game_over", "Das Spiel ist beendet.")
	}
	var offer *TradeOffer
	for _, t := range g.Trades {
		if t.ID == id {
			offer = t
			break
		}
	}
	if offer == nil || offer.Status != TradeOpen {
		return g.fail("trade_not_found", "Dieses Angebot ist nicht mehr offen.")
	}
	if offer.To != player {
		return g.fail("not_permitted", "Dieses Angebot richtet sich an einen anderen Spieler.")
	}
	if !accept {
		offer.Status = TradeDeclined
		g.logf("trade", player, 0, "%s lehnt den Handel ab.", g.Players[player].Name)
		return nil
	}
	from := offer.From
	to := offer.To
	if !g.active(from) || !g.active(to) {
		return g.fail("invalid_trade", "Ein Beteiligter ist nicht mehr im Spiel.")
	}
	if err := g.validateTradeSide(from, offer.OfferTiles, offer.OfferCash, offer.OfferCards); err != nil {
		return err
	}
	if err := g.validateTradeSide(to, offer.RequestTiles, offer.RequestCash, offer.RequestCards); err != nil {
		return err
	}
	if (from == g.Current || to == g.Current) && !g.activeTradeWindow() {
		return g.fail("wrong_phase", "Der aktive Spieler kann erst vor dem Würfeln handeln.")
	}

	g.Players[from].Cash -= offer.OfferCash
	g.Players[to].Cash += offer.OfferCash
	g.Players[to].Cash -= offer.RequestCash
	g.Players[from].Cash += offer.RequestCash
	for _, idx := range offer.OfferTiles {
		g.Tiles[idx].Owner = to
	}
	for _, idx := range offer.RequestTiles {
		g.Tiles[idx].Owner = from
	}
	if offer.OfferCards > 0 {
		moved := append([]string(nil), g.Players[from].JailCards[:offer.OfferCards]...)
		g.Players[from].JailCards = g.Players[from].JailCards[offer.OfferCards:]
		g.Players[to].JailCards = append(g.Players[to].JailCards, moved...)
	}
	if offer.RequestCards > 0 {
		moved := append([]string(nil), g.Players[to].JailCards[:offer.RequestCards]...)
		g.Players[to].JailCards = g.Players[to].JailCards[offer.RequestCards:]
		g.Players[from].JailCards = append(g.Players[from].JailCards, moved...)
	}
	g.Players[from].Stats.Trades++
	g.Players[to].Stats.Trades++
	offer.Status = TradeAccepted
	g.logf("trade", from, 0, "%s und %s einigen sich auf den Handel.", g.Players[from].Name, g.Players[to].Name)
	return nil
}
