package board

import (
	"slices"
	"testing"
)

func TestCatalogIsValid(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(cat.Tiles) != 40 {
		t.Fatalf("tiles = %d, want 40", len(cat.Tiles))
	}
	if len(cat.Cards) != 32 {
		t.Fatalf("cards = %d, want 32", len(cat.Cards))
	}
}

func TestBoardValues(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		name  string
		index int
		field string
		price int
		rent  []int
		build int
		sell  int
		group Group
	}{
		{name: "braun start", index: 1, field: "Zollgasse", price: 60, rent: []int{2, 10, 30, 90, 160, 250}, build: 50, sell: 42, group: GroupBraun},
		{name: "hellblau middle", index: 8, field: "Handelsweg", price: 100, rent: []int{6, 30, 90, 270, 400, 550}, build: 50, sell: 70, group: GroupHellblau},
		{name: "rosa", index: 14, field: "Rosengarten", price: 160, rent: []int{12, 60, 180, 500, 700, 900}, build: 100, sell: 112, group: GroupRosa},
		{name: "orange", index: 19, field: "Ulmenplatz", price: 200, rent: []int{16, 80, 220, 600, 800, 1000}, build: 100, sell: 140, group: GroupOrange},
		{name: "rot", index: 24, field: "Kontorplatz", price: 240, rent: []int{20, 100, 300, 750, 925, 1100}, build: 150, sell: 168, group: GroupRot},
		{name: "gelb", index: 29, field: "Dividendenring", price: 280, rent: []int{24, 120, 360, 850, 1025, 1200}, build: 150, sell: 196, group: GroupGelb},
		{name: "gruen", index: 34, field: "Fondsplatz", price: 320, rent: []int{28, 150, 450, 1000, 1200, 1400}, build: 200, sell: 224, group: GroupGruen},
		{name: "dunkelblau", index: 39, field: "Subprime-Allee", price: 400, rent: []int{50, 200, 600, 1400, 1700, 2000}, build: 200, sell: 280, group: GroupDunkelblau},
		{name: "tankstelle", index: 35, field: "Tankstelle Ost", price: 200, rent: []int{25, 50, 100, 200}, sell: 140, group: GroupTankstelle},
		{name: "utility", index: 28, field: "Wasserwerk", price: 150, sell: 105, group: GroupVersorger},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tile := cat.Tile(tc.index)
			if tile.Name != tc.field {
				t.Errorf("name = %q, want %q", tile.Name, tc.field)
			}
			if tile.Price != tc.price {
				t.Errorf("price = %d, want %d", tile.Price, tc.price)
			}
			if tile.Group != tc.group {
				t.Errorf("group = %q, want %q", tile.Group, tc.group)
			}
			if tile.Build != tc.build {
				t.Errorf("build = %d, want %d", tile.Build, tc.build)
			}
			if tile.Sell != tc.sell {
				t.Errorf("sell = %d, want %d", tile.Sell, tc.sell)
			}
			if !slices.Equal(tile.Rents, tc.rent) {
				t.Errorf("rents = %v, want %v", tile.Rents, tc.rent)
			}
		})
	}
}

func TestBoardLayout(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		index int
		typ   TileType
	}{
		{0, TypeGo}, {2, TypeChest}, {4, TypeTax}, {5, TypeTankstelle}, {7, TypeChance},
		{10, TypeJail}, {12, TypeUtility}, {17, TypeChest}, {20, TypeParking},
		{22, TypeChance}, {28, TypeUtility}, {30, TypeGotoJail}, {33, TypeChest},
		{36, TypeChance}, {38, TypeTax},
	}
	for _, tc := range tests {
		if got := cat.Tile(tc.index).Type; got != tc.typ {
			t.Errorf("tile %d type = %q, want %q", tc.index, got, tc.typ)
		}
	}
	if got := len(cat.GroupTiles(GroupTankstelle)); got != 4 {
		t.Errorf("tankstellen = %d, want 4", got)
	}
	if got := len(cat.GroupTiles(GroupVersorger)); got != 2 {
		t.Errorf("utilities = %d, want 2", got)
	}
}

func TestTaxFields(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	income := cat.Tile(4)
	if income.TaxPercent != 10 || income.Tax != 0 {
		t.Errorf("Einkommensteuer tax = %d, percent = %d; want 0 and 10", income.Tax, income.TaxPercent)
	}
	extra := cat.Tile(38)
	if extra.Tax != 100 || extra.TaxPercent != 0 {
		t.Errorf("Zusatzsteuer tax = %d, percent = %d; want 100 and 0", extra.Tax, extra.TaxPercent)
	}
}

func TestTankstelleFields(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		index int
		name  string
	}{
		{5, "Tankstelle Süd"},
		{15, "Tankstelle West"},
		{25, "Tankstelle Nord"},
		{35, "Tankstelle Ost"},
	}
	for _, tc := range tests {
		tile := cat.Tile(tc.index)
		if tile.Name != tc.name {
			t.Errorf("tile %d name = %q, want %q", tc.index, tile.Name, tc.name)
		}
		if tile.Type != TypeTankstelle {
			t.Errorf("tile %d type = %q, want %q", tc.index, tile.Type, TypeTankstelle)
		}
		if tile.Group != GroupTankstelle {
			t.Errorf("tile %d group = %q, want %q", tc.index, tile.Group, GroupTankstelle)
		}
		if tile.Price != 200 {
			t.Errorf("tile %d price = %d, want 200", tc.index, tile.Price)
		}
		if tile.Sell != 140 {
			t.Errorf("tile %d sell = %d, want 140", tc.index, tile.Sell)
		}
		if !slices.Equal(tile.Rents, []int{25, 50, 100, 200}) {
			t.Errorf("tile %d rents = %v, want [25 50 100 200]", tc.index, tile.Rents)
		}
		if tile.Build != 0 {
			t.Errorf("tile %d build = %d, want 0", tc.index, tile.Build)
		}
	}
}

func TestCards(t *testing.T) {
	cat, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	counts := map[Deck]int{}
	effects := map[Effect]bool{}
	for _, card := range cat.Cards {
		counts[card.Deck]++
		effects[card.Effect] = true
		if _, ok := cat.Card(card.ID); !ok {
			t.Errorf("card %q not lookupable", card.ID)
		}
	}
	if counts[DeckChance] != 16 {
		t.Errorf("chance cards = %d, want 16", counts[DeckChance])
	}
	if counts[DeckChest] != 16 {
		t.Errorf("chest cards = %d, want 16", counts[DeckChest])
	}
	want := []Effect{
		EffectAdvanceTo, EffectMoveBy, EffectGoToJail, EffectGetOutOfJail,
		EffectBankPayout, EffectBankPayment, EffectPayEachPlayer,
		EffectCollectFromEachPlayer, EffectPayPerHouse, EffectNextTankstelle, EffectNextUtility,
	}
	for _, effect := range want {
		if !effects[effect] {
			t.Errorf("effect %q missing from catalog", effect)
		}
	}
}

func TestInvalidCatalogRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Catalog)
	}{
		{
			name: "missing tile",
			mutate: func(c *Catalog) {
				c.Tiles = c.Tiles[:39]
			},
		},
		{
			name: "wrong rent ladder",
			mutate: func(c *Catalog) {
				c.Tiles = slices.Clone(c.Tiles)
				c.Tiles[1].Rents = []int{2, 10}
			},
		},
		{
			name: "wrong sell price",
			mutate: func(c *Catalog) {
				c.Tiles = slices.Clone(c.Tiles)
				c.Tiles[1].Sell = 50
			},
		},
		{
			name: "missing card",
			mutate: func(c *Catalog) {
				c.Cards = c.Cards[:31]
			},
		},
		{
			name: "invalid target",
			mutate: func(c *Catalog) {
				c.Cards = slices.Clone(c.Cards)
				c.Cards[0].Target = pos(40)
			},
		},
		{
			name: "tax without amount",
			mutate: func(c *Catalog) {
				c.Tiles = slices.Clone(c.Tiles)
				c.Tiles[4].TaxPercent = 0
			},
		},
		{
			name: "tax with fixed amount and percentage",
			mutate: func(c *Catalog) {
				c.Tiles = slices.Clone(c.Tiles)
				c.Tiles[4].Tax = 200
			},
		},
		{
			name: "tax percentage above 100",
			mutate: func(c *Catalog) {
				c.Tiles = slices.Clone(c.Tiles)
				c.Tiles[4].TaxPercent = 150
			},
		},
		{
			name: "negative money effect",
			mutate: func(c *Catalog) {
				c.Cards = slices.Clone(c.Cards)
				c.Cards[7].Amount = 0
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cat := &Catalog{Tiles: slices.Clone(Tiles), Cards: slices.Clone(Cards)}
			tc.mutate(cat)
			if err := cat.validate(); err == nil {
				t.Errorf("validate() = nil, want error")
			}
		})
	}
}
