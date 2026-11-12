// Package board holds the static Subprime board and card catalog (Spec.md §3.1,
// §3.2). The catalog is the single source of truth for the game; it is validated
// at startup and the server refuses to start on an invalid catalog.
package board

import "fmt"

// TileType is the kind of a board field.
type TileType string

const (
	TypeGo         TileType = "go"
	TypeStreet     TileType = "street"
	TypeTankstelle TileType = "tankstelle"
	TypeUtility    TileType = "utility"
	TypeTax        TileType = "tax"
	TypeChance     TileType = "chance"
	TypeChest      TileType = "chest"
	TypeJail       TileType = "jail"
	TypeGotoJail   TileType = "gotojail"
	TypeParking    TileType = "parking"
)

// Group is a color group or the tankstelle/utility group.
type Group string

const (
	GroupBraun      Group = "braun"
	GroupHellblau   Group = "hellblau"
	GroupRosa       Group = "rosa"
	GroupOrange     Group = "orange"
	GroupRot        Group = "rot"
	GroupGelb       Group = "gelb"
	GroupGruen      Group = "gruen"
	GroupDunkelblau Group = "dunkelblau"
	GroupTankstelle Group = "tankstelle"
	GroupVersorger  Group = "versorger"
)

// GroupSizes lists the fixed number of tiles per group.
var GroupSizes = map[Group]int{
	GroupBraun:      2,
	GroupHellblau:   3,
	GroupRosa:       3,
	GroupOrange:     3,
	GroupRot:        3,
	GroupGelb:       3,
	GroupGruen:      3,
	GroupDunkelblau: 2,
	GroupTankstelle: 4,
	GroupVersorger:  2,
}

// Tile is one field of the 40-field board. Rents holds [base, 1..4 houses,
// hotel]; the sell price is 70 % of the purchase price (rounded down).
type Tile struct {
	Index int      `json:"index"`
	Name  string   `json:"name"`
	Type  TileType `json:"type"`
	Group Group    `json:"group,omitempty"`
	Price int      `json:"price,omitempty"`
	Rents []int    `json:"rents,omitempty"`
	Build int      `json:"build,omitempty"`
	Sell  int      `json:"sell,omitempty"`
	Tax   int      `json:"tax,omitempty"`
}

// Buyable reports whether a tile can be owned.
func (t Tile) Buyable() bool {
	return t.Type == TypeStreet || t.Type == TypeTankstelle || t.Type == TypeUtility
}

// IsStreet reports whether the tile is a street (part of a color group).
func (t Tile) IsStreet() bool { return t.Type == TypeStreet }

// Deck identifies one of the two card decks.
type Deck string

const (
	DeckChance Deck = "ereignisfeld"
	DeckChest  Deck = "gemeinschaftsfeld"
)

// Effect is the mechanical effect of a card.
type Effect string

const (
	EffectAdvanceTo             Effect = "advance_to"
	EffectMoveBy                Effect = "move_by"
	EffectGoToJail              Effect = "go_to_jail"
	EffectGetOutOfJail          Effect = "get_out_of_jail"
	EffectBankPayout            Effect = "bank_payout"
	EffectBankPayment           Effect = "bank_payment"
	EffectPayEachPlayer         Effect = "pay_each_player"
	EffectCollectFromEachPlayer Effect = "collect_from_each_player"
	EffectPayPerHouse           Effect = "pay_per_house"
	EffectNextTankstelle        Effect = "next_tankstelle"
	EffectNextUtility           Effect = "next_utility"
)

// Card is one card of a deck. Target is authoritative and always a board
// position or a relative offset — never a street name.
type Card struct {
	ID        string `json:"id"`
	Deck      Deck   `json:"deck"`
	Text      string `json:"text"`
	Effect    Effect `json:"effect"`
	Target    *int   `json:"target,omitempty"`
	Amount    int    `json:"amount,omitempty"`
	PerHotel  int    `json:"perHotel,omitempty"`
	LosSalary bool   `json:"losSalary"`
}

// Catalog is the validated board + card data.
type Catalog struct {
	Tiles []Tile
	Cards []Card
	byID  map[string]*Tile
}

// Tiles is the fixed 40-field board in clockwise order, index 0 = Los.
var Tiles = []Tile{
	{Index: 0, Name: "Los", Type: TypeGo},
	{Index: 1, Name: "Zollgasse", Type: TypeStreet, Group: GroupBraun, Price: 60, Rents: []int{2, 10, 30, 90, 160, 250}, Build: 50, Sell: 42},
	{Index: 2, Name: "Gemeinschaftsfeld", Type: TypeChest},
	{Index: 3, Name: "Münzgasse", Type: TypeStreet, Group: GroupBraun, Price: 60, Rents: []int{4, 20, 60, 180, 320, 450}, Build: 50, Sell: 42},
	{Index: 4, Name: "Einkommensteuer", Type: TypeTax, Tax: 200},
	{Index: 5, Name: "Tankstelle Süd", Type: TypeTankstelle, Group: GroupTankstelle, Price: 200, Rents: []int{25, 50, 100, 200}, Sell: 140},
	{Index: 6, Name: "Marktstraße", Type: TypeStreet, Group: GroupHellblau, Price: 100, Rents: []int{6, 30, 90, 270, 400, 550}, Build: 50, Sell: 70},
	{Index: 7, Name: "Ereignisfeld", Type: TypeChance},
	{Index: 8, Name: "Handelsweg", Type: TypeStreet, Group: GroupHellblau, Price: 100, Rents: []int{6, 30, 90, 270, 400, 550}, Build: 50, Sell: 70},
	{Index: 9, Name: "Uferpromenade", Type: TypeStreet, Group: GroupHellblau, Price: 120, Rents: []int{8, 40, 100, 300, 450, 600}, Build: 50, Sell: 84},
	{Index: 10, Name: "Werkstatt", Type: TypeJail},
	{Index: 11, Name: "Gartenstraße", Type: TypeStreet, Group: GroupRosa, Price: 140, Rents: []int{10, 50, 150, 450, 625, 750}, Build: 100, Sell: 98},
	{Index: 12, Name: "Elektrizitätswerk", Type: TypeUtility, Group: GroupVersorger, Price: 150, Sell: 105},
	{Index: 13, Name: "Lindenplatz", Type: TypeStreet, Group: GroupRosa, Price: 140, Rents: []int{10, 50, 150, 450, 625, 750}, Build: 100, Sell: 98},
	{Index: 14, Name: "Rosengarten", Type: TypeStreet, Group: GroupRosa, Price: 160, Rents: []int{12, 60, 180, 500, 700, 900}, Build: 100, Sell: 112},
	{Index: 15, Name: "Tankstelle West", Type: TypeTankstelle, Group: GroupTankstelle, Price: 200, Rents: []int{25, 50, 100, 200}, Sell: 140},
	{Index: 16, Name: "Ahornring", Type: TypeStreet, Group: GroupOrange, Price: 180, Rents: []int{14, 70, 200, 550, 750, 950}, Build: 100, Sell: 126},
	{Index: 17, Name: "Gemeinschaftsfeld", Type: TypeChest},
	{Index: 18, Name: "Eichenweg", Type: TypeStreet, Group: GroupOrange, Price: 180, Rents: []int{14, 70, 200, 550, 750, 950}, Build: 100, Sell: 126},
	{Index: 19, Name: "Ulmenplatz", Type: TypeStreet, Group: GroupOrange, Price: 200, Rents: []int{16, 80, 220, 600, 800, 1000}, Build: 100, Sell: 140},
	{Index: 20, Name: "Frei-Parken", Type: TypeParking},
	{Index: 21, Name: "Bankstraße", Type: TypeStreet, Group: GroupRot, Price: 220, Rents: []int{18, 90, 250, 700, 875, 1050}, Build: 150, Sell: 154},
	{Index: 22, Name: "Ereignisfeld", Type: TypeChance},
	{Index: 23, Name: "Zinsgasse", Type: TypeStreet, Group: GroupRot, Price: 220, Rents: []int{18, 90, 250, 700, 875, 1050}, Build: 150, Sell: 154},
	{Index: 24, Name: "Kontorplatz", Type: TypeStreet, Group: GroupRot, Price: 240, Rents: []int{20, 100, 300, 750, 925, 1100}, Build: 150, Sell: 168},
	{Index: 25, Name: "Tankstelle Nord", Type: TypeTankstelle, Group: GroupTankstelle, Price: 200, Rents: []int{25, 50, 100, 200}, Sell: 140},
	{Index: 26, Name: "Anleiheweg", Type: TypeStreet, Group: GroupGelb, Price: 260, Rents: []int{22, 110, 330, 800, 975, 1150}, Build: 150, Sell: 182},
	{Index: 27, Name: "Renditenstraße", Type: TypeStreet, Group: GroupGelb, Price: 260, Rents: []int{22, 110, 330, 800, 975, 1150}, Build: 150, Sell: 182},
	{Index: 28, Name: "Wasserwerk", Type: TypeUtility, Group: GroupVersorger, Price: 150, Sell: 105},
	{Index: 29, Name: "Dividendenring", Type: TypeStreet, Group: GroupGelb, Price: 280, Rents: []int{24, 120, 360, 850, 1025, 1200}, Build: 150, Sell: 196},
	{Index: 30, Name: "Unfall", Type: TypeGotoJail},
	{Index: 31, Name: "Börsenplatz", Type: TypeStreet, Group: GroupGruen, Price: 300, Rents: []int{26, 130, 390, 900, 1100, 1275}, Build: 200, Sell: 210},
	{Index: 32, Name: "Depotstraße", Type: TypeStreet, Group: GroupGruen, Price: 300, Rents: []int{26, 130, 390, 900, 1100, 1275}, Build: 200, Sell: 210},
	{Index: 33, Name: "Gemeinschaftsfeld", Type: TypeChest},
	{Index: 34, Name: "Fondsplatz", Type: TypeStreet, Group: GroupGruen, Price: 320, Rents: []int{28, 150, 450, 1000, 1200, 1400}, Build: 200, Sell: 224},
	{Index: 35, Name: "Tankstelle Ost", Type: TypeTankstelle, Group: GroupTankstelle, Price: 200, Rents: []int{25, 50, 100, 200}, Sell: 140},
	{Index: 36, Name: "Ereignisfeld", Type: TypeChance},
	{Index: 37, Name: "Kreditstraße", Type: TypeStreet, Group: GroupDunkelblau, Price: 350, Rents: []int{35, 175, 500, 1100, 1300, 1500}, Build: 200, Sell: 245},
	{Index: 38, Name: "Zusatzsteuer", Type: TypeTax, Tax: 100},
	{Index: 39, Name: "Subprime-Allee", Type: TypeStreet, Group: GroupDunkelblau, Price: 400, Rents: []int{50, 200, 600, 1400, 1700, 2000}, Build: 200, Sell: 280},
}

func pos(v int) *int { return &v }

// Cards is the fixed 32-card catalog (Spec.md §3.2.1). The informal ("du")
// variant is used throughout.
var Cards = []Card{
	// Ereignisfeld (chance)
	{ID: "chance-advance-los", Deck: DeckChance, Text: "Rücke vor bis auf Los, ziehe 200 GE ein.", Effect: EffectAdvanceTo, Target: pos(0), LosSalary: true},
	{ID: "chance-go-to-jail", Deck: DeckChance, Text: "Du hast einen Unfall: Begib dich direkt in die Werkstatt. Gehe nicht über Los. Ziehe nicht 200 GE ein.", Effect: EffectGoToJail, Target: pos(10)},
	{ID: "chance-jail-free", Deck: DeckChance, Text: "Pannenhilfe: Du kommst gratis aus der Werkstatt. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst.", Effect: EffectGetOutOfJail},
	{ID: "chance-back-three", Deck: DeckChance, Text: "Gehe drei Felder zurück.", Effect: EffectMoveBy, Target: pos(-3)},
	{ID: "chance-next-tankstelle-1", Deck: DeckChance, Text: "Rücke vor bis zur nächsten Tankstelle. Wenn die Tankstelle noch niemandem gehört, kannst du sie von der Bank kaufen. Falls sie einen Besitzer hat, zahle ihm das Doppelte der normalen Miete.", Effect: EffectNextTankstelle},
	{ID: "chance-next-tankstelle-2", Deck: DeckChance, Text: "Rücke vor bis zur nächsten Tankstelle. Gehört die Tankstelle niemandem, kannst du sie von der Bank kaufen. Hat sie einen Besitzer, zahlst du ihm das Doppelte der normalen Miete.", Effect: EffectNextTankstelle},
	{ID: "chance-next-utility", Deck: DeckChance, Text: "Rücke vor bis zum nächsten Versorgungswerk. Gehört das Werk niemandem, kannst du es von der Bank kaufen. Sonst wirfst du die Würfel und zahlst dem Eigentümer das Zehnfache deines Würfelergebnisses.", Effect: EffectNextUtility},
	{ID: "chance-dividend", Deck: DeckChance, Text: "Die Bank zahlt dir eine Dividende von 50 GE.", Effect: EffectBankPayout, Amount: 50},
	{ID: "chance-trip-tankstelle-sued", Deck: DeckChance, Text: "Mache einen Ausflug zur Tankstelle Süd. Wenn du über Los kommst, ziehst du 200 GE ein.", Effect: EffectAdvanceTo, Target: pos(5), LosSalary: true},
	{ID: "chance-gartenstrasse", Deck: DeckChance, Text: "Rücke vor bis zur Gartenstraße. Wenn du über Los kommst, ziehst du 200 GE ein.", Effect: EffectAdvanceTo, Target: pos(11), LosSalary: true},
	{ID: "chance-kontorplatz", Deck: DeckChance, Text: "Rücke vor bis zum Kontorplatz. Wenn du über Los kommst, ziehst du 200 GE ein.", Effect: EffectAdvanceTo, Target: pos(24), LosSalary: true},
	{ID: "chance-subprime-allee", Deck: DeckChance, Text: "Rücke vor bis zur Subprime-Allee.", Effect: EffectAdvanceTo, Target: pos(39), LosSalary: true},
	{ID: "chance-bauspar", Deck: DeckChance, Text: "Dein Bausparvertrag wird fällig. Du erhältst 150 GE.", Effect: EffectBankPayout, Amount: 150},
	{ID: "chance-speeding", Deck: DeckChance, Text: "Strafe für zu schnelles Fahren. Zahle 15 GE.", Effect: EffectBankPayment, Amount: 15},
	{ID: "chance-vorstand", Deck: DeckChance, Text: "Du bist zum Vorstand gewählt worden. Zahle jedem Spieler 50 GE.", Effect: EffectPayEachPlayer, Amount: 50},
	{ID: "chance-renovation", Deck: DeckChance, Text: "Du lässt deine Häuser renovieren. Zahle 25 GE pro Haus und 100 GE pro Hotel.", Effect: EffectPayPerHouse, Amount: 25, PerHotel: 100},

	// Gemeinschaftsfeld (chest)
	{ID: "chest-jail-free", Deck: DeckChest, Text: "Pannenhilfe: Du kommst gratis aus der Werkstatt. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst.", Effect: EffectGetOutOfJail},
	{ID: "chest-go-to-jail", Deck: DeckChest, Text: "Du hast einen Unfall: Begib dich direkt in die Werkstatt. Gehe nicht über Los. Ziehe nicht 200 GE ein.", Effect: EffectGoToJail, Target: pos(10)},
	{ID: "chest-advance-los", Deck: DeckChest, Text: "Rücke vor bis auf Los, ziehe 200 GE ein.", Effect: EffectAdvanceTo, Target: pos(0), LosSalary: true},
	{ID: "chest-bank-error", Deck: DeckChest, Text: "Bank-Irrtum zu deinen Gunsten. Ziehe 200 GE ein.", Effect: EffectBankPayout, Amount: 200},
	{ID: "chest-inherit", Deck: DeckChest, Text: "Du erbst 100 GE.", Effect: EffectBankPayout, Amount: 100},
	{ID: "chest-life-insurance", Deck: DeckChest, Text: "Deine Lebensversicherung wird fällig. Du erhältst 100 GE.", Effect: EffectBankPayout, Amount: 100},
	{ID: "chest-warehouse-sale", Deck: DeckChest, Text: "Aus Lagerverkäufen erhältst du 50 GE.", Effect: EffectBankPayout, Amount: 50},
	{ID: "chest-consulting-fee", Deck: DeckChest, Text: "Du erhältst eine Beratungsgebühr von 25 GE.", Effect: EffectBankPayout, Amount: 25},
	{ID: "chest-tax-refund", Deck: DeckChest, Text: "Einkommensteuerrückzahlung. Du erhältst 20 GE.", Effect: EffectBankPayout, Amount: 20},
	{ID: "chest-drag-race", Deck: DeckChest, Text: "Du hast in einem Drag Race gewonnen. Du erhältst 10 GE.", Effect: EffectBankPayout, Amount: 10},
	{ID: "chest-birthday", Deck: DeckChest, Text: "Du hast Geburtstag! Jeder Spieler schenkt dir 10 GE.", Effect: EffectCollectFromEachPlayer, Amount: 10},
	{ID: "chest-vacation", Deck: DeckChest, Text: "Urlaubsgeld! Du erhältst 100 GE.", Effect: EffectBankPayout, Amount: 100},
	{ID: "chest-school-fee", Deck: DeckChest, Text: "Zahle Schulgeld. Zahle 50 GE.", Effect: EffectBankPayment, Amount: 50},
	{ID: "chest-street-repair", Deck: DeckChest, Text: "Kosten für Straßenausbesserungen! Zahle 40 GE pro Haus und 115 GE pro Hotel.", Effect: EffectPayPerHouse, Amount: 40, PerHotel: 115},
	{ID: "chest-hospital", Deck: DeckChest, Text: "Krankenhausgebühren. Zahle 100 GE.", Effect: EffectBankPayment, Amount: 100},
	{ID: "chest-doctor", Deck: DeckChest, Text: "Arztkosten. Zahle 50 GE.", Effect: EffectBankPayment, Amount: 50},
}

// New validates and returns the catalog.
func New() (*Catalog, error) {
	c := &Catalog{Tiles: Tiles, Cards: Cards, byID: make(map[string]*Tile, len(Tiles))}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// Tile returns the tile at index i.
func (c *Catalog) Tile(i int) Tile { return c.Tiles[i] }

// Card returns the card with the given id.
func (c *Catalog) Card(id string) (Card, bool) {
	for _, card := range c.Cards {
		if card.ID == id {
			return card, true
		}
	}
	return Card{}, false
}

// GroupTiles returns the indices of all tiles in a group.
func (c *Catalog) GroupTiles(g Group) []int {
	out := make([]int, 0, GroupSizes[g])
	for _, t := range c.Tiles {
		if t.Group == g {
			out = append(out, t.Index)
		}
	}
	return out
}

// UtilityIndex returns the index of the named utility.
func (c *Catalog) UtilityIndex(name string) int {
	for _, t := range c.Tiles {
		if t.Type == TypeUtility && t.Name == name {
			return t.Index
		}
	}
	return -1
}

func (c *Catalog) validate() error {
	if len(c.Tiles) != 40 {
		return fmt.Errorf("board has %d tiles, want 40", len(c.Tiles))
	}
	groups := map[Group]int{}
	for i, t := range c.Tiles {
		if t.Index != i {
			return fmt.Errorf("tile %d has index %d", i, t.Index)
		}
		if t.Name == "" {
			return fmt.Errorf("tile %d has no name", i)
		}
		switch t.Type {
		case TypeGo, TypeStreet, TypeTankstelle, TypeUtility, TypeTax, TypeChance, TypeChest, TypeJail, TypeGotoJail, TypeParking:
		default:
			return fmt.Errorf("tile %d has unknown type %q", i, t.Type)
		}
		if t.Buyable() {
			if t.Price <= 0 {
				return fmt.Errorf("tile %d has non-positive price", i)
			}
			if t.Sell != t.Price*70/100 {
				return fmt.Errorf("tile %d sell price = %d, want %d", i, t.Sell, t.Price*70/100)
			}
			if _, ok := GroupSizes[t.Group]; !ok {
				return fmt.Errorf("tile %d has unknown group %q", i, t.Group)
			}
			groups[t.Group]++
			if t.Type == TypeStreet {
				if len(t.Rents) != 6 || t.Build <= 0 {
					return fmt.Errorf("tile %d has invalid street data", i)
				}
				for _, r := range t.Rents {
					if r <= 0 {
						return fmt.Errorf("tile %d has non-positive rent", i)
					}
				}
			}
		}
	}
	for g, want := range GroupSizes {
		if groups[g] != want {
			return fmt.Errorf("group %q has %d tiles, want %d", g, groups[g], want)
		}
	}

	counts := map[Deck]int{}
	ids := map[string]bool{}
	for _, card := range c.Cards {
		if card.ID == "" || ids[card.ID] {
			return fmt.Errorf("card %q has an empty or duplicate id", card.ID)
		}
		ids[card.ID] = true
		if card.Text == "" {
			return fmt.Errorf("card %q has no text", card.ID)
		}
		if card.Deck != DeckChance && card.Deck != DeckChest {
			return fmt.Errorf("card %q has unknown deck %q", card.ID, card.Deck)
		}
		counts[card.Deck]++
		switch card.Effect {
		case EffectAdvanceTo:
			if card.Target == nil || *card.Target < 0 || *card.Target > 39 {
				return fmt.Errorf("card %q has invalid target", card.ID)
			}
		case EffectMoveBy:
			if card.Target == nil || *card.Target == 0 || *card.Target < -39 || *card.Target > 39 {
				return fmt.Errorf("card %q has invalid offset", card.ID)
			}
		case EffectGoToJail:
			if card.Target == nil || *card.Target != 10 {
				return fmt.Errorf("card %q must target the jail", card.ID)
			}
		case EffectGetOutOfJail, EffectNextTankstelle, EffectNextUtility:
			if card.Target != nil {
				return fmt.Errorf("card %q must not have a target", card.ID)
			}
		case EffectBankPayout, EffectBankPayment, EffectPayEachPlayer, EffectCollectFromEachPlayer:
			if card.Amount <= 0 {
				return fmt.Errorf("card %q has non-positive amount", card.ID)
			}
		case EffectPayPerHouse:
			if card.Amount <= 0 || card.PerHotel <= 0 {
				return fmt.Errorf("card %q has invalid per-house/per-hotel amount", card.ID)
			}
		default:
			return fmt.Errorf("card %q has unknown effect %q", card.ID, card.Effect)
		}
	}
	for _, deck := range []Deck{DeckChance, DeckChest} {
		if counts[deck] != 16 {
			return fmt.Errorf("deck %q has %d cards, want 16", deck, counts[deck])
		}
	}
	return nil
}
