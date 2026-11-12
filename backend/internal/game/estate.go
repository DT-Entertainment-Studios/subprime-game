package game

// requireEstateWindow restricts building, mortgages and bank sales to the
// player's pre-turn window (Spec.md §4.6, §4.7).
func (g *Game) requireEstateWindow(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	return g.requireTurnPhase(TurnAwaitingRoll)
}

// requireMoneyWindow allows selling assets in the normal pre-turn window, while
// the active player is settling a debt, while they decide on a purchase they
// cannot afford yet (Spec.md §4.4), and while they are choosing a jail option
// so they can raise the repair costs (Spec.md §4.10).
func (g *Game) requireMoneyWindow(player int) error {
	if err := g.requireTurn(player); err != nil {
		return err
	}
	switch g.TurnPhase {
	case TurnAwaitingRoll, TurnAwaitingDebt, TurnAwaitingPurchase, TurnAwaitingJail:
		return nil
	}
	return g.requireTurnPhase(TurnAwaitingRoll)
}

// Build adds a house (or a hotel on the fifth house) to an owned street.
func (g *Game) Build(player, idx int) error {
	if err := g.requireEstateWindow(player); err != nil {
		return err
	}
	if idx < 0 || idx >= len(g.Tiles) {
		return g.fail("invalid_target", "Unbekanntes Feld.")
	}
	t := g.tile(idx)
	st := &g.Tiles[idx]
	if !t.IsStreet() || st.Owner != player {
		return g.fail("not_owner", "Du besitzt dieses Feld nicht.")
	}
	if st.Mortgaged {
		return g.fail("mortgaged", "Ein hypothekiertes Feld kann nicht bebaut werden.")
	}
	if !g.ownsAllStreets(player, t.Group) {
		return g.fail("not_complete", "Du besitzt nicht die ganze Farbgruppe.")
	}
	for _, other := range g.groupTiles(t.Group) {
		if g.Tiles[other].Mortgaged {
			return g.fail("mortgaged", "In dieser Farbgruppe ist ein Feld hypothekiert.")
		}
	}
	if st.Houses >= 5 {
		return g.fail("limit", "Dieses Feld hat bereits ein Hotel.")
	}
	minLevel := 5
	for _, other := range g.groupTiles(t.Group) {
		if g.Tiles[other].Houses < minLevel {
			minLevel = g.Tiles[other].Houses
		}
	}
	if st.Houses > minLevel {
		return g.fail("even_build", "Es muss gleichmäßig gebaut werden.")
	}
	if g.Players[player].Cash < t.Build {
		return g.fail("insufficient_cash", "Du kannst den Bau nicht bezahlen.")
	}
	g.Players[player].Cash -= t.Build
	st.Houses++
	g.Players[player].Stats.HousesBuilt++
	if st.Houses == 5 {
		g.logf("build", player, t.Build, "%s baut ein Hotel auf %s.", g.Players[player].Name, t.Name)
	} else {
		g.logf("build", player, t.Build, "%s baut ein Haus auf %s (%d).", g.Players[player].Name, t.Name, st.Houses)
	}
	return nil
}

// SellBuilding sells one house or a hotel back to the bank (Spec.md §4.6).
func (g *Game) SellBuilding(player, idx int) error {
	if err := g.requireMoneyWindow(player); err != nil {
		return err
	}
	if idx < 0 || idx >= len(g.Tiles) {
		return g.fail("invalid_target", "Unbekanntes Feld.")
	}
	t := g.tile(idx)
	st := &g.Tiles[idx]
	if st.Owner != player {
		return g.fail("not_owner", "Du besitzt dieses Feld nicht.")
	}
	if st.Houses == 0 {
		return g.fail("no_building", "Auf diesem Feld steht kein Gebäude.")
	}
	refund := t.Build / 2
	if st.Houses == 5 {
		st.Houses = 4
	} else {
		st.Houses--
	}
	g.Players[player].Cash += refund
	g.logf("sell_building", player, refund, "%s verkauft ein Gebäude auf %s für %d GE.", g.Players[player].Name, t.Name, refund)
	g.settleDebtIfCleared()
	return nil
}

// Mortgage mortgages an unbuilt property for 50 % of its price.
func (g *Game) Mortgage(player, idx int) error {
	if err := g.requireMoneyWindow(player); err != nil {
		return err
	}
	if idx < 0 || idx >= len(g.Tiles) {
		return g.fail("invalid_target", "Unbekanntes Feld.")
	}
	t := g.tile(idx)
	st := &g.Tiles[idx]
	if st.Owner != player || !t.Buyable() {
		return g.fail("not_owner", "Du besitzt dieses Feld nicht.")
	}
	if st.Mortgaged {
		return g.fail("mortgaged", "Das Feld ist bereits hypothekiert.")
	}
	if st.Houses > 0 || (t.Group != "" && g.groupHasBuildings(t.Group)) {
		return g.fail("has_buildings", "Die Farbgruppe muss frei von Gebäuden sein.")
	}
	amount := t.Price / 2
	st.Mortgaged = true
	g.Players[player].Cash += amount
	g.logf("mortgage", player, amount, "%s hypothekiert %s für %d GE.", g.Players[player].Name, t.Name, amount)
	g.settleDebtIfCleared()
	return nil
}

// Unmortgage redeems a mortgage for 60 % of the purchase price.
func (g *Game) Unmortgage(player, idx int) error {
	if err := g.requireEstateWindow(player); err != nil {
		return err
	}
	if idx < 0 || idx >= len(g.Tiles) {
		return g.fail("invalid_target", "Unbekanntes Feld.")
	}
	t := g.tile(idx)
	st := &g.Tiles[idx]
	if st.Owner != player || !t.Buyable() {
		return g.fail("not_owner", "Du besitzt dieses Feld nicht.")
	}
	if !st.Mortgaged {
		return g.fail("not_mortgaged", "Das Feld ist nicht hypothekiert.")
	}
	cost := t.Price * 60 / 100
	if g.Players[player].Cash < cost {
		return g.fail("insufficient_cash", "Du kannst die Hypothek nicht ablösen.")
	}
	g.Players[player].Cash -= cost
	st.Mortgaged = false
	g.logf("unmortgage", player, cost, "%s löst die Hypothek auf %s für %d GE ab.", g.Players[player].Name, t.Name, cost)
	return nil
}

// SellToBank sells an unbuilt property for 70 % of its price; a mortgaged
// property returns to the bank for nothing.
func (g *Game) SellToBank(player, idx int) error {
	if err := g.requireMoneyWindow(player); err != nil {
		return err
	}
	if idx < 0 || idx >= len(g.Tiles) {
		return g.fail("invalid_target", "Unbekanntes Feld.")
	}
	t := g.tile(idx)
	st := &g.Tiles[idx]
	if st.Owner != player || !t.Buyable() {
		return g.fail("not_owner", "Du besitzt dieses Feld nicht.")
	}
	if st.Houses > 0 {
		return g.fail("has_buildings", "Gebäude müssen zuerst verkauft werden.")
	}
	amount := 0
	if !st.Mortgaged {
		amount = t.Sell
	}
	g.Players[player].Cash += amount
	g.Players[player].Stats.PropertiesSold++
	*st = TileState{Owner: Bank}
	g.logf("sell_to_bank", player, amount, "%s verkauft %s an die Bank für %d GE.", g.Players[player].Name, t.Name, amount)
	g.settleDebtIfCleared()
	return nil
}
