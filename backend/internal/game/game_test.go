package game

import (
	"math/rand"
	"testing"
	"time"

	"github.com/DT-Entertainment-Studios/subprime-game/backend/internal/board"
)

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time          { return c.now }
func (c *testClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestGame(t *testing.T, count int) (*Game, *testClock) {
	t.Helper()
	cat, err := board.New()
	if err != nil {
		t.Fatalf("board.New() error = %v", err)
	}
	players := make([]*Player, count)
	colors := []string{"rot", "blau", "gruen", "gelb", "rosa", "orange", "lila", "schwarz"}
	for i := range players {
		players[i] = &Player{ID: string(rune('a' + i)), Name: string(rune('A' + i)), Color: colors[i]}
	}
	clock := &testClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	g, err := New(cat, players, Options{
		Rand: rand.New(rand.NewSource(7)),
		Now:  clock.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return g, clock
}

func setDice(g *Game, a, b int) {
	g.dice = func() (int, int) { return a, b }
}

func TestRentLadders(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Tiles[1].Owner = 0
	g.Tiles[5].Owner = 0
	g.Tiles[15].Owner = 0
	g.Tiles[12].Owner = 1
	g.Tiles[28].Owner = 1
	g.Dice = [2]int{3, 4}
	g.Tiles[11].Owner = 0
	g.Tiles[13].Owner = 0
	g.Tiles[14].Owner = 0

	tests := []struct {
		name  string
		index int
		setup func()
		want  int
	}{
		{name: "single street", index: 1, want: 2},
		{name: "complete group doubles unbuilt rent", index: 1, setup: func() { g.Tiles[3].Owner = 0 }, want: 4},
		{name: "one house", index: 1, setup: func() { g.Tiles[1].Houses = 1 }, want: 10},
		{name: "hotel", index: 1, setup: func() { g.Tiles[1].Houses = 5 }, want: 250},
		{name: "tankstelle with two owned", index: 5, want: 50},
		{name: "utility with both owned", index: 12, want: 70},
		{name: "mortgaged street", index: 13, setup: func() { g.Tiles[13].Mortgaged = true }, want: 0},
		{name: "owner in jail", index: 14, setup: func() { g.Players[0].InJail = true }, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup()
			}
			if got := g.rentAmount(tc.index); got != tc.want {
				t.Errorf("rentAmount(%d) = %d, want %d", tc.index, got, tc.want)
			}
		})
	}
}

func TestUtilityShare(t *testing.T) {
	tests := []struct {
		name        string
		utilityOwn  int // player index, Bank for unowned
		rent        int
		wantOwner   int
		wantUtility int
		wantEvents  int
	}{
		{name: "to player rounds down", utilityOwn: 1, rent: 15, wantOwner: 1514, wantUtility: 1501, wantEvents: 1},
		{name: "unowned utility keeps full rent", utilityOwn: Bank, rent: 15, wantOwner: 1515, wantUtility: 1500, wantEvents: 0},
		{name: "same owner keeps the share", utilityOwn: 0, rent: 15, wantOwner: 1515, wantUtility: 1500, wantEvents: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			g.Tiles[11].Owner = 0 // Gartenstraße, rosa
			g.Tiles[12].Owner = tc.utilityOwn
			g.chargeRent(2, 0, 11, tc.rent)
			if got := g.Players[0].Cash; got != tc.wantOwner {
				t.Errorf("owner cash = %d, want %d", got, tc.wantOwner)
			}
			if got := g.Players[1].Cash; got != tc.wantUtility {
				t.Errorf("utility cash = %d, want %d", got, tc.wantUtility)
			}
			if got := countEvents(g, "utility_share"); got != tc.wantEvents {
				t.Errorf("utility_share events = %d, want %d", got, tc.wantEvents)
			}
		})
	}
}

func TestUtilityShareMortgageAndJail(t *testing.T) {
	tests := []struct {
		name        string
		street      int
		utility     int
		utilityOwn  int
		mortgaged   bool
		ownerInJail bool
		wantOwner   int
		wantUtility int
	}{
		{name: "electricity mortgaged", street: 11, utility: 12, utilityOwn: 1, mortgaged: true, wantOwner: 1515, wantUtility: 1500},
		{name: "electricity owner in jail", street: 11, utility: 12, utilityOwn: 1, ownerInJail: true, wantOwner: 1515, wantUtility: 1500},
		{name: "water mortgaged", street: 21, utility: 28, utilityOwn: 1, mortgaged: true, wantOwner: 1515, wantUtility: 1500},
		{name: "water owner in jail", street: 21, utility: 28, utilityOwn: 1, ownerInJail: true, wantOwner: 1515, wantUtility: 1500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			g.Tiles[tc.street].Owner = 0
			g.Tiles[tc.utility].Owner = tc.utilityOwn
			g.Tiles[tc.utility].Mortgaged = tc.mortgaged
			if tc.utilityOwn != Bank {
				g.Players[tc.utilityOwn].InJail = tc.ownerInJail
			}
			g.chargeRent(2, 0, tc.street, 15)
			if got := g.Players[0].Cash; got != tc.wantOwner {
				t.Errorf("owner cash = %d, want %d", got, tc.wantOwner)
			}
			if got := g.Players[1].Cash; got != tc.wantUtility {
				t.Errorf("utility cash = %d, want %d", got, tc.wantUtility)
			}
			if got := countEvents(g, "utility_share"); got != 0 {
				t.Errorf("utility_share events = %d, want 0", got)
			}
		})
	}
}

func TestUtilityShareGroups(t *testing.T) {
	tests := []struct {
		name      string
		street    int
		utility   int
		wantShare int
	}{
		{name: "rosa pays electricity", street: 11, utility: 12, wantShare: 1},
		{name: "orange pays electricity", street: 16, utility: 12, wantShare: 1},
		{name: "rot pays water", street: 21, utility: 28, wantShare: 1},
		{name: "gelb pays water", street: 26, utility: 28, wantShare: 1},
		{name: "hellblau pays nothing", street: 6, utility: 12, wantShare: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			g.Tiles[tc.street].Owner = 0
			g.Tiles[tc.utility].Owner = 1
			g.chargeRent(2, 0, tc.street, 15)
			if got := g.Players[1].Cash - 1500; got != tc.wantShare {
				t.Errorf("share = %d, want %d", got, tc.wantShare)
			}
		})
	}
}

func TestPurchaseDoublePrice(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[3].Owner = 0
	g.offerPurchase(0, 1)
	if g.Pending == nil || !g.Pending.Doubled || g.Pending.Price != 120 {
		t.Fatalf("pending = %+v, want doubled 120", g.Pending)
	}
	if err := g.Buy(0); err != nil {
		t.Fatalf("Buy() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 1380 {
		t.Errorf("cash = %d, want 1380", got)
	}
	if g.Tiles[1].Owner != 0 {
		t.Errorf("owner = %d, want 0", g.Tiles[1].Owner)
	}
}

func TestUnaffordablePurchaseWaitsForPlayer(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].Cash = 10
	g.Tiles[6].Owner = 0
	g.offerPurchase(0, 1)
	if g.Auction != nil {
		t.Fatal("auction must not start automatically")
	}
	if g.Pending == nil || g.Pending.Price != 60 || g.TurnPhase != TurnAwaitingPurchase {
		t.Fatalf("pending = %+v phase = %q, want offer for 60/awaiting_purchase", g.Pending, g.TurnPhase)
	}
	if err := g.Buy(0); err == nil {
		t.Fatal("Buy() = nil, want error while cash is short")
	}
	if err := g.Build(0, 6); err == nil {
		t.Fatal("Build() = nil, want error during a purchase decision")
	}
	if err := g.Mortgage(0, 6); err != nil {
		t.Fatalf("Mortgage() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 60 {
		t.Fatalf("cash after mortgage = %d, want 60", got)
	}
	if err := g.Buy(0); err != nil {
		t.Fatalf("Buy() error = %v", err)
	}
	if g.Tiles[1].Owner != 0 {
		t.Errorf("owner = %d, want 0", g.Tiles[1].Owner)
	}
	if g.Auction != nil {
		t.Error("auction started although the player bought the tile")
	}
}

func TestAuction(t *testing.T) {
	tests := []struct {
		name      string
		bid       int
		ownsBrown bool
		wantOwner int
		wantCash  int
		wantBank  bool
	}{
		{name: "regular win", bid: 100, wantOwner: 1, wantCash: 1400},
		{name: "below catalog price", bid: 40, wantOwner: 1, wantCash: 1460},
		{name: "completing group pays 120 percent", bid: 60, ownsBrown: true, wantOwner: 1, wantCash: 1428},
		{name: "no bid stays with bank", bid: 0, wantBank: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, clock := newTestGame(t, 2)
			if tc.ownsBrown {
				g.Tiles[3].Owner = 1
			}
			g.offerPurchase(0, 1)
			if err := g.Decline(0); err != nil {
				t.Fatalf("Decline() error = %v", err)
			}
			if g.Auction == nil {
				t.Fatal("auction not started")
			}
			if tc.bid > 0 {
				if err := g.Bid(1, tc.bid); err != nil {
					t.Fatalf("Bid() error = %v", err)
				}
			}
			clock.Advance(16 * time.Second)
			if !g.Tick(clock.Now()) {
				t.Fatal("Tick() = false, want true")
			}
			if tc.wantBank {
				if g.Tiles[1].Owner != Bank {
					t.Errorf("owner = %d, want bank", g.Tiles[1].Owner)
				}
				return
			}
			if g.Tiles[1].Owner != tc.wantOwner {
				t.Errorf("owner = %d, want %d", g.Tiles[1].Owner, tc.wantOwner)
			}
			if got := g.Players[1].Cash; got != tc.wantCash {
				t.Errorf("cash = %d, want %d", got, tc.wantCash)
			}
		})
	}
}

func TestAuctionBidValidation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, g *Game)
		player  int
		amount  int
		wantErr bool
	}{
		{name: "zero bid", player: 1, amount: 0, wantErr: true},
		{name: "not a multiple of 20", player: 1, amount: 70, wantErr: true},
		{name: "own bid", setup: func(t *testing.T, g *Game) {
			if err := g.Bid(1, 60); err != nil {
				t.Fatalf("Bid(1, 60) error = %v", err)
			}
		}, player: 1, amount: 80, wantErr: true},
		{name: "bid must exceed current bid", setup: func(t *testing.T, g *Game) {
			if err := g.Bid(1, 60); err != nil {
				t.Fatalf("Bid(1, 60) error = %v", err)
			}
		}, player: 0, amount: 60, wantErr: true},
		{name: "below catalog price is allowed", player: 1, amount: 40},
		{name: "minimum bid is allowed", player: 1, amount: 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 2)
			g.offerPurchase(0, 1)
			if err := g.Decline(0); err != nil {
				t.Fatalf("Decline() error = %v", err)
			}
			if tc.setup != nil {
				tc.setup(t, g)
			}
			err := g.Bid(tc.player, tc.amount)
			if tc.wantErr && err == nil {
				t.Errorf("Bid(%d, %d) = nil, want error", tc.player, tc.amount)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Bid(%d, %d) error = %v", tc.player, tc.amount, err)
			}
		})
	}
}

func TestAuctionBidResetsTimer(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration // time between auction start and the bid
	}{
		{name: "more than five seconds remaining", elapsed: 5 * time.Second},
		{name: "less than five seconds remaining", elapsed: 12 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, clock := newTestGame(t, 2)
			g.offerPurchase(0, 1)
			if err := g.Decline(0); err != nil {
				t.Fatalf("Decline() error = %v", err)
			}
			if g.Auction == nil {
				t.Fatal("auction not started")
			}
			clock.Advance(tc.elapsed)
			if err := g.Bid(1, 100); err != nil {
				t.Fatalf("Bid() error = %v", err)
			}
			want := clock.Now().Add(time.Duration(g.Rules.AuctionSeconds) * time.Second)
			if !g.Auction.EndsAt.Equal(want) {
				t.Errorf("EndsAt = %v, want %v (%d s left after every bid)", g.Auction.EndsAt, want, g.Rules.AuctionSeconds)
			}
		})
	}
}

func TestMortgageAndSale(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	if err := g.Mortgage(0, 1); err != nil {
		t.Fatalf("Mortgage() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 1530 {
		t.Errorf("cash after mortgage = %d, want 1530", got)
	}
	if err := g.Unmortgage(0, 1); err != nil {
		t.Fatalf("Unmortgage() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 1494 {
		t.Errorf("cash after unmortgage = %d, want 1494", got)
	}
	if err := g.SellToBank(0, 1); err != nil {
		t.Fatalf("SellToBank() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 1536 {
		t.Errorf("cash after sale = %d, want 1536", got)
	}
	if g.Tiles[1].Owner != Bank {
		t.Errorf("owner = %d, want bank", g.Tiles[1].Owner)
	}
}

func TestMortgageRequiresBuildingFreeGroup(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[3].Owner = 0
	g.Tiles[3].Houses = 1
	if err := g.Mortgage(0, 1); err == nil {
		t.Error("Mortgage() = nil, want error while group has buildings")
	}
}

func TestEvenBuilding(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[3].Owner = 0
	if err := g.Build(0, 1); err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	if err := g.Build(0, 1); err == nil {
		t.Error("second Build() on same tile = nil, want error")
	}
	if err := g.Build(0, 3); err != nil {
		t.Fatalf("Build() on other tile error = %v", err)
	}
	if err := g.Build(0, 1); err != nil {
		t.Fatalf("Build() back on first tile error = %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := g.Build(0, 3); err != nil {
			t.Fatalf("Build() tile 3 iteration %d error = %v", i, err)
		}
		if err := g.Build(0, 1); err != nil {
			t.Fatalf("Build() tile 1 iteration %d error = %v", i, err)
		}
	}
	if err := g.Build(0, 3); err != nil {
		t.Fatalf("Build() tile 3 error = %v", err)
	}
	// Both tiles now have four houses.
	if err := g.Build(0, 1); err != nil {
		t.Fatalf("hotel Build() error = %v", err)
	}
	if g.Tiles[1].Houses != 5 {
		t.Fatalf("houses = %d, want hotel", g.Tiles[1].Houses)
	}
	cashBefore := g.Players[0].Cash
	if err := g.SellBuilding(0, 1); err != nil {
		t.Fatalf("SellBuilding() error = %v", err)
	}
	if g.Tiles[1].Houses != 4 {
		t.Errorf("houses = %d, want 4", g.Tiles[1].Houses)
	}
	if got := g.Players[0].Cash - cashBefore; got != 25 {
		t.Errorf("refund = %d, want 25", got)
	}
}

func TestDebtInsteadOfBankruptcy(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[1].Houses = 1
	g.Players[0].Cash = 5
	received := g.settle(0, map[int]int{1: 500})
	if g.Players[0].Bankrupt {
		t.Fatal("player went bankrupt instead of into debt")
	}
	if got := g.Players[0].Cash; got != -495 {
		t.Errorf("cash = %d, want -495", got)
	}
	if got := received[1]; got != 500 {
		t.Errorf("creditor received = %d, want 500", got)
	}
	if g.Players[1].Cash != 2000 {
		t.Errorf("creditor cash = %d, want 2000", g.Players[1].Cash)
	}
	if g.Tiles[1].Owner != 0 || g.Tiles[1].Houses != 1 {
		t.Errorf("tile changed: %+v", g.Tiles[1])
	}
	if g.TurnPhase != TurnAwaitingDebt {
		t.Errorf("turn phase = %q, want %q", g.TurnPhase, TurnAwaitingDebt)
	}
}

func TestMultiCreditorDebt(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Cash = 11
	received := g.settle(0, map[int]int{1: 50, 2: 50})
	if got := g.Players[0].Cash; got != -89 {
		t.Errorf("debtor cash = %d, want -89", got)
	}
	if received[1] != 50 || received[2] != 50 {
		t.Errorf("shares = %d/%d, want 50/50", received[1], received[2])
	}
	if g.Players[1].Cash != 1550 || g.Players[2].Cash != 1550 {
		t.Errorf("creditor cash = %d/%d, want 1550/1550", g.Players[1].Cash, g.Players[2].Cash)
	}
}

func TestDebtBlocksPlayAndMortgageClearsIt(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 1 // Zollgasse, rent 2
	g.Tiles[3].Owner = 0 // Münzgasse, mortgage value 30
	g.Players[0].Cash = 1
	setDice(g, 1, 0) // lands on 1, cannot cover the rent
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	if g.Current != 0 || g.TurnPhase != TurnAwaitingDebt {
		t.Fatalf("current = %d phase = %q, want 0/awaiting_debt", g.Current, g.TurnPhase)
	}
	if got := g.Players[0].Cash; got != -1 {
		t.Errorf("cash = %d, want -1", got)
	}
	if err := g.Roll(0); err == nil {
		t.Error("Roll() while in debt = nil, want error")
	}
	if err := g.Mortgage(0, 3); err != nil {
		t.Fatalf("Mortgage() error = %v", err)
	}
	if got := g.Players[0].Cash; got != 29 {
		t.Errorf("cash after mortgage = %d, want 29", got)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestAdvanceTurnStartsDebtPhase(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[1].Cash = -5
	g.advanceTurn()
	if g.Current != 1 || g.TurnPhase != TurnAwaitingDebt {
		t.Errorf("current = %d phase = %q, want 1/awaiting_debt", g.Current, g.TurnPhase)
	}
}

func TestGiveUpWhenInDebt(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[1].Houses = 1
	g.Players[0].Cash = -10
	g.TurnPhase = TurnAwaitingDebt
	if err := g.GiveUp(0); err != nil {
		t.Fatalf("GiveUp() error = %v", err)
	}
	if !g.Players[0].Bankrupt {
		t.Error("player is not bankrupt after giving up")
	}
	if g.Tiles[1].Owner != Bank || g.Tiles[1].Houses != 0 {
		t.Errorf("tile not returned to bank: %+v", g.Tiles[1])
	}
	if g.Phase != PhaseFinished || g.Winner != 1 {
		t.Errorf("phase = %q winner = %d, want finished/1", g.Phase, g.Winner)
	}
}

func TestGiveUpRequiresDebt(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].Cash = 10
	if err := g.GiveUp(0); err == nil {
		t.Error("GiveUp() without debt = nil, want error")
	}
}

func TestForfeitWithoutDebt(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Cash = 700
	g.Tiles[1].Owner = 0
	g.Tiles[1].Houses = 2
	g.Tiles[3].Owner = 0
	g.Tiles[3].Mortgaged = true
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if !g.Players[0].Bankrupt {
		t.Error("player is not bankrupt after forfeiting")
	}
	if got := g.Players[0].Cash; got != 0 {
		t.Errorf("cash = %d, want 0", got)
	}
	for _, idx := range []int{1, 3} {
		if g.Tiles[idx].Owner != Bank || g.Tiles[idx].Houses != 0 {
			t.Errorf("tile %d not returned to bank: %+v", idx, g.Tiles[idx])
		}
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestForfeitDuringPurchaseDiscardsOffer(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Cash = 100
	setDice(g, 1, 0)
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	if g.TurnPhase != TurnAwaitingPurchase || g.Pending == nil {
		t.Fatalf("turn phase = %q pending = %+v, want awaiting_purchase with an offer", g.TurnPhase, g.Pending)
	}
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if g.Pending != nil {
		t.Errorf("pending = %+v after forfeiting, want nil", g.Pending)
	}
	if g.Auction != nil {
		t.Errorf("auction = %+v after forfeiting, want nil", g.Auction)
	}
	if g.Tiles[1].Owner != Bank {
		t.Errorf("tile 1 owner = %d, want bank", g.Tiles[1].Owner)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestForfeitDuringJail(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].InJail = true
	g.Players[0].JailAttempts = 1
	g.TurnPhase = TurnAwaitingJail
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if !g.Players[0].Bankrupt {
		t.Error("player is not bankrupt after forfeiting")
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestForfeitEndsGame(t *testing.T) {
	g, _ := newTestGame(t, 2)
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if g.Phase != PhaseFinished || g.Winner != 1 {
		t.Errorf("phase = %q winner = %d, want finished/1", g.Phase, g.Winner)
	}
	if g.TurnPhase != TurnFinished {
		t.Errorf("turn phase = %q, want %q", g.TurnPhase, TurnFinished)
	}
	if g.Pending != nil {
		t.Errorf("pending = %+v after the game ended, want nil", g.Pending)
	}
}

func TestForfeitConsumesExtraTurn(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.extraTurn = true
	g.TurnPhase = TurnAwaitingPurchase
	g.Pending = &PendingPurchase{Tile: 1, Price: 60}
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if g.Current != 1 {
		t.Errorf("current = %d, want 1 (the double must not grant a reroll)", g.Current)
	}
}

func TestForfeitLapsesTrades(t *testing.T) {
	g, _ := newTestGame(t, 3)
	if _, err := g.ProposeTrade(0, TradeInput{To: 1, OfferCash: 10, RequestCash: 5}); err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if err := g.Forfeit(0); err != nil {
		t.Fatalf("Forfeit() error = %v", err)
	}
	if len(g.Trades) != 1 || g.Trades[0].Status != TradeLapsed {
		t.Errorf("trades = %+v, want one lapsed offer", g.Trades)
	}
}

func TestForfeitRejectsInvalidPhase(t *testing.T) {
	tests := []struct {
		name   string
		player int
		setup  func(g *Game)
		code   string
	}{
		{
			name:   "debt phase",
			player: 0,
			setup: func(g *Game) {
				g.Players[0].Cash = -10
				g.TurnPhase = TurnAwaitingDebt
			},
			code: "wrong_phase",
		},
		{
			name:   "auction",
			player: 0,
			setup: func(g *Game) {
				g.TurnPhase = TurnAuction
				g.Auction = &Auction{Tile: 1, MinBid: 10}
			},
			code: "wrong_phase",
		},
		{
			name:   "not the current player",
			player: 1,
			setup:  func(g *Game) {},
			code:   "not_your_turn",
		},
		{
			name:   "finished game",
			player: 0,
			setup:  func(g *Game) { g.Phase = PhaseFinished },
			code:   "game_over",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, _ := newTestGame(t, 3)
			tc.setup(g)
			err := g.Forfeit(tc.player)
			re, ok := err.(*RuleError)
			if !ok {
				t.Fatalf("Forfeit() error = %v (%T), want *RuleError", err, err)
			}
			if re.Code != tc.code {
				t.Errorf("code = %q, want %q", re.Code, tc.code)
			}
		})
	}
}

func TestLeaveDuringPurchaseClearsOffer(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Players[0].Connected = false
	setDice(g, 1, 0)
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	if g.TurnPhase != TurnAwaitingPurchase || g.Pending == nil {
		t.Fatalf("turn phase = %q pending = %+v, want awaiting_purchase with an offer", g.TurnPhase, g.Pending)
	}
	if err := g.Leave(0); err != nil {
		t.Fatalf("Leave() error = %v", err)
	}
	if g.Pending != nil {
		t.Errorf("pending = %+v after leaving, want nil", g.Pending)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestCardBankPayoutAndRecycling(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Decks[board.DeckChest].Draw = []string{"chest-bank-error", "chest-doctor"}
	g.drawAndApply(0, board.DeckChest, 0)
	if got := g.Players[0].Cash; got != 1700 {
		t.Errorf("cash = %d, want 1700", got)
	}
	draw := g.Decks[board.DeckChest].Draw
	if len(draw) != 2 || draw[len(draw)-1] != "chest-bank-error" {
		t.Errorf("deck = %v, want used card at bottom", draw)
	}
}

func TestCardEventCarriesDeck(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Decks[board.DeckChest].Draw = []string{"chest-bank-error"}
	g.drawAndApply(0, board.DeckChest, 0)
	var card *Event
	for i := range g.Events {
		if g.Events[i].Type == "card" {
			card = &g.Events[i]
		}
	}
	if card == nil {
		t.Fatal("no card event logged")
	}
	if card.Deck != board.DeckChest {
		t.Errorf("deck = %q, want %q", card.Deck, board.DeckChest)
	}
}

func TestCardAdvanceLos(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].Position = 36
	g.Decks[board.DeckChance].Draw = []string{"chance-advance-los"}
	g.drawAndApply(0, board.DeckChance, 0)
	if g.Players[0].Position != 0 {
		t.Errorf("position = %d, want 0", g.Players[0].Position)
	}
	if got := g.Players[0].Cash; got != 1700 {
		t.Errorf("cash = %d, want 1700", got)
	}
}

func TestMoveEventsCarryDirection(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.moveForward(0, 3, false)
	if got := g.Events[len(g.Events)-1].Type; got != "move" {
		t.Errorf("forward move event = %q, want move", got)
	}
	g.moveBackward(0, 3)
	if got := g.Events[len(g.Events)-1].Type; got != "move_back" {
		t.Errorf("backward move event = %q, want move_back", got)
	}
	if got := g.Players[0].Position; got != 0 {
		t.Errorf("position = %d, want 0", got)
	}
}

func TestJailFreeCardKeptAndUsed(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Decks[board.DeckChest].Draw = []string{"chest-jail-free"}
	g.drawAndApply(0, board.DeckChest, 0)
	if len(g.Players[0].JailCards) != 1 {
		t.Fatalf("jail cards = %d, want 1", len(g.Players[0].JailCards))
	}
	for _, id := range g.Decks[board.DeckChest].Draw {
		if id == "chest-jail-free" {
			t.Error("jail-free card was returned while held")
		}
	}
	g.sendToJail(0)
	g.TurnPhase = TurnAwaitingJail
	setDice(g, 1, 2)
	if err := g.JailOption(0, JailOptionCard); err != nil {
		t.Fatalf("JailOption(card) error = %v", err)
	}
	if len(g.Players[0].JailCards) != 0 {
		t.Errorf("jail cards = %d, want 0", len(g.Players[0].JailCards))
	}
	if g.Players[0].InJail {
		t.Error("player still in jail")
	}
}

func TestJailThirdAttemptCostsFine(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].InJail = true
	g.Players[0].Position = 10
	g.Players[0].JailAttempts = 2
	g.Tiles[13].Owner = 0
	g.TurnPhase = TurnAwaitingJail
	setDice(g, 1, 2)
	if err := g.JailOption(0, JailOptionRoll); err != nil {
		t.Fatalf("JailOption(roll) error = %v", err)
	}
	if got := g.Players[0].Cash; got != 1450 {
		t.Errorf("cash = %d, want 1450", got)
	}
	if g.Players[0].Position != 13 {
		t.Errorf("position = %d, want 13", g.Players[0].Position)
	}
	if g.Current != 1 {
		t.Errorf("current = %d, want 1", g.Current)
	}
}

func TestJailAllowsMortgageToRaiseFine(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].InJail = true
	g.Players[0].Position = 10
	g.Players[0].Cash = 30
	g.Tiles[1].Owner = 0
	g.TurnPhase = TurnAwaitingJail
	if err := g.Mortgage(0, 1); err != nil {
		t.Fatalf("Mortgage() in jail error = %v", err)
	}
	if got := g.Players[0].Cash; got != 60 {
		t.Fatalf("cash after mortgage = %d, want 60", got)
	}
	setDice(g, 1, 2)
	if err := g.JailOption(0, JailOptionPay); err != nil {
		t.Fatalf("JailOption(pay) error = %v", err)
	}
	if g.Players[0].InJail {
		t.Error("player still in jail after paying the fine")
	}
	if got := g.Players[0].Cash; got != 10 {
		t.Errorf("cash after fine = %d, want 10", got)
	}
}

func TestJailDoubleMovesWithoutExtraTurn(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Players[0].InJail = true
	g.Players[0].Position = 10
	g.Tiles[14].Owner = 0
	g.TurnPhase = TurnAwaitingJail
	setDice(g, 2, 2)
	if err := g.JailOption(0, JailOptionRoll); err != nil {
		t.Fatalf("JailOption(roll) error = %v", err)
	}
	if g.Players[0].Position != 14 {
		t.Errorf("position = %d, want 14", g.Players[0].Position)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestRollAndBuyAdvancesTurn(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Decks[board.DeckChest].Draw = []string{"chest-bank-error"}
	setDice(g, 1, 1)
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	if g.Current != 0 || g.TurnPhase != TurnAwaitingRoll {
		t.Fatalf("double should grant another turn, current = %d phase = %q", g.Current, g.TurnPhase)
	}
	setDice(g, 1, 2)
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	if g.TurnPhase != TurnAwaitingPurchase {
		t.Fatalf("turn phase = %q, want awaiting_purchase", g.TurnPhase)
	}
	if err := g.Buy(0); err != nil {
		t.Fatalf("Buy() error = %v", err)
	}
	if g.Current != 1 || g.TurnPhase != TurnAwaitingRoll {
		t.Errorf("current = %d phase = %q, want 1/awaiting_roll", g.Current, g.TurnPhase)
	}
}

func TestRentInJailPaysNothing(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 1
	g.Players[1].InJail = true
	g.Players[0].Position = 0
	setDice(g, 1, 2) // lands on 3? 0+3 = 3, owned by nobody; use direct charge
	g.chargeRent(0, 1, 1, g.rentAmount(1))
	if g.Players[0].Cash != 1500 || g.Players[1].Cash != 1500 {
		t.Errorf("rent was charged while owner in jail: %d/%d", g.Players[0].Cash, g.Players[1].Cash)
	}
}

func TestTradeAccept(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[5].Owner = 1
	id, err := g.ProposeTrade(0, TradeInput{
		To: 1, OfferTiles: []int{1}, RequestTiles: []int{5}, OfferCash: 20, RequestCash: 10,
	})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	if err := g.RespondTrade(1, id, true); err != nil {
		t.Fatalf("RespondTrade() error = %v", err)
	}
	if g.Tiles[1].Owner != 1 || g.Tiles[5].Owner != 0 {
		t.Errorf("tiles not transferred: %d/%d", g.Tiles[1].Owner, g.Tiles[5].Owner)
	}
	if g.Players[0].Cash != 1490 || g.Players[1].Cash != 1510 {
		t.Errorf("cash = %d/%d, want 1490/1510", g.Players[0].Cash, g.Players[1].Cash)
	}
}

func TestTradeRejectsBuiltTile(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[3].Owner = 0
	g.Tiles[1].Houses = 1
	if _, err := g.ProposeTrade(0, TradeInput{To: 1, OfferTiles: []int{1}}); err == nil {
		t.Error("ProposeTrade() = nil, want error for built tile")
	}
}

func TestTradeLapsesOnRoll(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[5].Owner = 1
	id, err := g.ProposeTrade(0, TradeInput{To: 1, RequestTiles: []int{5}, OfferCash: 10})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	setDice(g, 1, 2)
	if err := g.Roll(0); err != nil {
		t.Fatalf("Roll() error = %v", err)
	}
	for _, tr := range g.Trades {
		if tr.ID == id && tr.Status != TradeLapsed {
			t.Errorf("status = %q, want lapsed", tr.Status)
		}
	}
}

func TestTradeAllowedWhileInJail(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[5].Owner = 1
	g.Players[0].InJail = true
	g.Players[0].Position = 10
	g.TurnPhase = TurnAwaitingJail

	// The jailed active player may propose a trade before the jail roll.
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferTiles: []int{1}, RequestTiles: []int{5}})
	if err != nil {
		t.Fatalf("ProposeTrade() in jail error = %v", err)
	}
	// ... and the opponent may answer it.
	if err := g.RespondTrade(1, id, true); err != nil {
		t.Fatalf("RespondTrade() in jail error = %v", err)
	}
	if g.Tiles[1].Owner != 1 || g.Tiles[5].Owner != 0 {
		t.Errorf("tiles not transferred: %d/%d", g.Tiles[1].Owner, g.Tiles[5].Owner)
	}
	if !g.Players[0].InJail {
		t.Error("player left jail by trading")
	}
}

func TestTradeLapsesOnJailRoll(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[5].Owner = 1
	g.Players[0].InJail = true
	g.Players[0].Position = 10
	g.TurnPhase = TurnAwaitingJail
	id, err := g.ProposeTrade(0, TradeInput{To: 1, RequestTiles: []int{5}, OfferCash: 10})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	setDice(g, 1, 2)
	if err := g.JailOption(0, JailOptionRoll); err != nil {
		t.Fatalf("JailOption(roll) error = %v", err)
	}
	for _, tr := range g.Trades {
		if tr.ID == id && tr.Status != TradeLapsed {
			t.Errorf("status = %q, want lapsed", tr.Status)
		}
	}
}

func TestTradeCounterSupersedesOffer(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Tiles[1].Owner = 0
	g.Tiles[5].Owner = 1
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferTiles: []int{1}, RequestTiles: []int{5}, OfferCash: 20})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	// The recipient counters with the sides swapped.
	counterID, err := g.ProposeTrade(1, TradeInput{
		To: 0, OfferTiles: []int{5}, RequestTiles: []int{1}, RequestCash: 20, CounterTo: id,
	})
	if err != nil {
		t.Fatalf("ProposeTrade() counter error = %v", err)
	}
	for _, tr := range g.Trades {
		if tr.ID == id && tr.Status != TradeCountered {
			t.Errorf("original status = %q, want countered", tr.Status)
		}
	}
	if err := g.RespondTrade(1, id, true); err == nil {
		t.Error("RespondTrade() on countered offer = nil, want error")
	}
	if err := g.RespondTrade(0, counterID, true); err != nil {
		t.Fatalf("RespondTrade() counter error = %v", err)
	}
	if g.Tiles[1].Owner != 1 || g.Tiles[5].Owner != 0 {
		t.Errorf("tiles not transferred: %d/%d", g.Tiles[1].Owner, g.Tiles[5].Owner)
	}
}

func TestTradeCounterRejectsForeignOffer(t *testing.T) {
	g, _ := newTestGame(t, 3)
	g.Tiles[1].Owner = 0
	g.Tiles[5].Owner = 1
	id, err := g.ProposeTrade(0, TradeInput{To: 1, OfferTiles: []int{1}, RequestTiles: []int{5}})
	if err != nil {
		t.Fatalf("ProposeTrade() error = %v", err)
	}
	// A third player may not counter an offer he is not part of.
	if _, err := g.ProposeTrade(2, TradeInput{To: 1, RequestCash: 1, CounterTo: id}); err == nil {
		t.Error("ProposeTrade() counter by third player = nil, want error")
	}
	// An unknown counter id must not touch the open offer.
	if _, err := g.ProposeTrade(1, TradeInput{To: 0, RequestCash: 1, CounterTo: "missing"}); err == nil {
		t.Error("ProposeTrade() counter with unknown id = nil, want error")
	}
	for _, tr := range g.Trades {
		if tr.ID == id && tr.Status != TradeOpen {
			t.Errorf("status changed to %q", tr.Status)
		}
	}
}

func TestThreeDoublesSendToJail(t *testing.T) {
	g, _ := newTestGame(t, 2)
	g.Decks[board.DeckChest].Draw = []string{"chest-bank-error", "chest-bank-error", "chest-bank-error"}
	setDice(g, 1, 1)
	for i := 0; i < 3; i++ {
		if err := g.Roll(0); err != nil {
			t.Fatalf("Roll() %d error = %v", i, err)
		}
	}
	if !g.Players[0].InJail || g.Players[0].Position != 10 {
		t.Errorf("player not in jail: %+v", g.Players[0])
	}
	if g.Current == 0 {
		t.Error("turn did not pass after third double")
	}
}
