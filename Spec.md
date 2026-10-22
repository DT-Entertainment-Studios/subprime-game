# Subprime — Specification

Status: v1.0 draft · all open questions resolved (see section 9, Decision Log) ·
remaining veto-able assumptions in section 10.

## 1. Overview

Subprime is a self-hosted online multiplayer board game for a group of friends. It is a
Monopoly-like game with the classic 40-field board and the additional "Subprime" house
rules: buying the last street of a color group costs double, part of every rent flows to
the utilities, and crooked deals ("Krumme Deals") are forbidden. One player creates a
room, the others join with a short code or a link, and the server keeps the authoritative
game state in memory and pushes it to all participants in real time over WebSocket. The
entire UI is German.

The name comes from the financial term: everyone plays a real-estate shark trying to
outlast the others.

## 2. Users & Authentication

- Multi-user; every participant has an identity taken from the OAuth2 proxy in front of
  the app. There is no local login, registration, or password.
- The proxy is the only ingress (network isolation); the backend trusts the identity
  headers it receives.
- Identity headers (default names, configurable via environment variables):
  - `X-Forwarded-User` — stable external user id (required; missing or blank ⇒ HTTP 401)
  - `X-Forwarded-Preferred-Username` — display name shown on the board
  - `X-Forwarded-Email` — optional, shown in the user tooltip
  - `X-Forwarded-Groups` — comma- or whitespace-separated group list
- Users are provisioned lazily on the first authenticated request (JIT) and held in
  memory only. There is no database: a user exists as long as the process lives.
- User preferences (theme, board zoom, sound, animations, last used token color, reduced
  motion) are stored in the browser's local storage and never sent to the server.
- Display name and token color are chosen per game and never leave the server except
  through the game state.

## 3. Domain Model

All monetary amounts are integers in GE ("Geld-Einheiten"), the unit used in the rules.

### 3.1 Board catalog

The board is the classic 40-field board with the classic German color groups. The catalog
is the single source of truth for the game and lives in the backend
(`backend/internal/board/`): a static, hand-maintained Go source file in the repository,
not user-editable at runtime and not loaded from an external file at startup.

Tile types: `go`, `street`, `station`, `utility`, `tax`, `chance`, `chest`, `jail`,
`gotojail`, `parking`.

The street names are **renamed** (neutral, thematically fitting, free of third-party
trademarks); the color group names stay German because the rules refer to them
(`rosa` = pink, `orange`, `rot`, `gelb`).

Field layout (index 0 = "Los", clockwise; `rent` = [base, 1 house, 2 houses, 3 houses,
4 houses, hotel]; `build` = cost per house; the mortgage value is always 50 % of the
purchase price and is therefore not a separate column — only the 70 % sell-back price is
listed):

| # | Field | Type | Group | Price | Rent | Build | Sell to bank |
|---|-------|------|-------|-------|------|-------|--------------|
| 0 | Los | go | — | — | — | — | — |
| 1 | Zollgasse | street | braun | 60 | 2, 10, 30, 90, 160, 250 | 50 | 42 |
| 2 | Gemeinschaftsfeld | chest | — | — | — | — | — |
| 3 | Münzgasse | street | braun | 60 | 4, 20, 60, 180, 320, 450 | 50 | 42 |
| 4 | Einkommensteuer | tax | — | — | 200 | — | — |
| 5 | Fernbahnhof | station | bahnhof | 200 | 25, 50, 100, 200 | — | 140 |
| 6 | Marktstraße | street | hellblau | 100 | 6, 30, 90, 270, 400, 550 | 50 | 70 |
| 7 | Ereignisfeld | chance | — | — | — | — | — |
| 8 | Handelsweg | street | hellblau | 100 | 6, 30, 90, 270, 400, 550 | 50 | 70 |
| 9 | Uferpromenade | street | hellblau | 120 | 8, 40, 100, 300, 450, 600 | 50 | 84 |
| 10 | Gefängnis | jail | — | — | — | — | — |
| 11 | Gartenstraße | street | rosa | 140 | 10, 50, 150, 450, 625, 750 | 100 | 98 |
| 12 | Elektrizitätswerk | utility | versorger | 150 | 4× dice, 10× dice with both | — | 105 |
| 13 | Lindenplatz | street | rosa | 140 | 10, 50, 150, 450, 625, 750 | 100 | 98 |
| 14 | Rosengarten | street | rosa | 160 | 12, 60, 180, 500, 700, 900 | 100 | 112 |
| 15 | Güterbahnhof | station | bahnhof | 200 | 25, 50, 100, 200 | — | 140 |
| 16 | Ahornring | street | orange | 180 | 14, 70, 200, 550, 750, 950 | 100 | 126 |
| 17 | Gemeinschaftsfeld | chest | — | — | — | — | — |
| 18 | Eichenweg | street | orange | 180 | 14, 70, 200, 550, 750, 950 | 100 | 126 |
| 19 | Ulmenplatz | street | orange | 200 | 16, 80, 220, 600, 800, 1000 | 100 | 140 |
| 20 | Frei-Parken | parking | — | — | — | — | — |
| 21 | Bankstraße | street | rot | 220 | 18, 90, 250, 700, 875, 1050 | 150 | 154 |
| 22 | Ereignisfeld | chance | — | — | — | — | — |
| 23 | Zinsgasse | street | rot | 220 | 18, 90, 250, 700, 875, 1050 | 150 | 154 |
| 24 | Kontorplatz | street | rot | 240 | 20, 100, 300, 750, 925, 1100 | 150 | 168 |
| 25 | Ringbahnhof | station | bahnhof | 200 | 25, 50, 100, 200 | — | 140 |
| 26 | Anleiheweg | street | gelb | 260 | 22, 110, 330, 800, 975, 1150 | 150 | 182 |
| 27 | Renditenstraße | street | gelb | 260 | 22, 110, 330, 800, 975, 1150 | 150 | 182 |
| 28 | Wasserwerk | utility | versorger | 150 | 4× dice, 10× dice with both | — | 105 |
| 29 | Dividendenring | street | gelb | 280 | 24, 120, 360, 850, 1025, 1200 | 150 | 196 |
| 30 | Zu Gefängnis | gotojail | — | — | — | — | — |
| 31 | Börsenplatz | street | grün | 300 | 26, 130, 390, 900, 1100, 1275 | 200 | 210 |
| 32 | Depotstraße | street | grün | 300 | 26, 130, 390, 900, 1100, 1275 | 200 | 210 |
| 33 | Gemeinschaftsfeld | chest | — | — | — | — | — |
| 34 | Fondsplatz | street | grün | 320 | 28, 150, 450, 1000, 1200, 1400 | 200 | 224 |
| 35 | Küstenbahnhof | station | bahnhof | 200 | 25, 50, 100, 200 | — | 140 |
| 36 | Ereignisfeld | chance | — | — | — | — | — |
| 37 | Kreditstraße | street | dunkelblau | 350 | 35, 175, 500, 1100, 1300, 1500 | 200 | 245 |
| 38 | Zusatzsteuer | tax | — | — | 100 | — | — |
| 39 | Subprime-Allee | street | dunkelblau | 400 | 50, 200, 600, 1400, 1700, 2000 | 200 | 280 |

Group sizes: braun 2, hellblau 3, rosa 3, orange 3, rot 3, gelb 3, grün 3, dunkelblau 2,
bahnhof 4, versorger 2.

Color-group naming follows the **international / post-2008 German** board: the first group
is `braun` (on the pre-2008 classic German board it was printed violet — an edition
difference, not a data conflict; the group is the two cheapest streets either way). The
groups the Subprime rules refer to — `rosa`, `orange`, `rot`, `gelb` — are unambiguous in
every edition.

The values are the classic board values; the names are the renamed neutral ones. Renaming
a street is a one-line data change in the catalog.

### 3.2 Cards

Two decks of 16 cards each, drawn from a field of type `chance` or `chest`. The board
fields are named **Ereignisfeld** (chance, positions 7/22/36) and **Gemeinschaftsfeld**
(chest, positions 2/17/33) — the names printed on the physical German board and generic
German words, consistent with the renamed streets.

- **Ereignisfeld** (chance): movement cards, "go to jail", "get out of jail free", bank
  payouts, and bank payments.
- **Gemeinschaftsfeld** (chest): bank payouts and bank payments, movement to specific
  fields, "get out of jail free", "go to jail".

Card handling in Subprime:

- **Every card payment goes to the bank** (never to another player) and every card payout
  comes from the bank.
- **Exception — the two classic player-to-player cards keep their classic behaviour**:
  "Du hast Geburtstag! Jeder Spieler schenkt dir 10 GE" (every other player gives you
  10 GE) and "Du bist zum Vorstand geworden. Zahle jedem Spieler 50 GE" (you pay every
  other player 50 GE). The bank rule covers card *payments*, not card *collections*.
- **Cards that name a street are rewritten to use the renamed streets** and are stored as
  data next to the card catalog, so a street rename stays a one-place change. Card targets
  are resolved **by position**, never by name.
- **Two cards keep their own rent rule** instead of the normal rent:
  - "advance to the next station": if owned, pay the owner **double** the normal rent; if
    unowned, the player may buy it from the bank.
  - "advance to the next utility": if owned, roll the dice and pay the owner **10×** the
    dice total; if unowned, the player may buy it from the bank.
- A drawn card is executed immediately and then placed face down **under** the pile. The
  "get out of jail free" card is **kept by the player** while held; when used it goes back
  under the pile.
- **Deck exhaustion**: when the draw pile is empty, the used cards are shuffled together
  and form a new draw pile.
- The **get-out-of-jail card is tradable** like a property: it is offered in the trade
  engine alongside street cards and sold for cash.

#### 3.2.1 The 32 cards

The decks are the classic German decks, with the street names rewritten to the renamed
board and the currency changed from `M` to GE. Targets are given as **board positions**
(0 = Los); the position is authoritative, the street name in the text is display only.

**Ereignisfeld** (chance) — drawn at positions 7, 22, 36:

| # | Card text (German) | Effect | Target |
|---|--------------------|--------|--------|
| 1 | "Rücke vor bis auf Los, ziehe 200 GE ein." | Advance to Los, collect the salary | 0 |
| 2 | "Gehe in das Gefängnis, begib dich direkt dorthin. Gehe nicht über Los. Ziehe nicht 200 GE ein." | Go to jail, no Los salary, turn ends | 10 |
| 3 | "Du kommst aus dem Gefängnis frei. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst." | Keep the card; use it to leave jail or sell it | — |
| 4 | "Gehe drei Felder zurück." | Move back 3 tiles and resolve that tile (no Los salary) | −3 |
| 5 | "Rücke vor bis zum nächsten Bahnhof. Wenn der Bahnhof noch niemanden gehört, kannst du ihn von der Bank kaufen. Falls er einen Besitzer hat, zahle ihm das Doppelte der normalen Miete." | Advance to the next station; buy it if unowned, otherwise pay double rent | next station |
| 6 | "Rücke vor bis zum nächsten Bahnhof. Gehört der Bahnhof niemandem, kannst du ihn von der Bank kaufen. Hat er einen Besitzer, zahlst du ihm das Doppelte der normalen Miete." | Same as card 5 — the second nearest-station card | next station |
| 7 | "Rücke vor bis zum nächsten Versorgungswerk. Gehört das Werk niemandem, kannst du es von der Bank kaufen. Sonst wirfst du die Würfel und zahlst dem Eigentümer das Zehnfache deines Würfelergebnisses." | Advance to the next utility; buy it if unowned, otherwise pay 10× dice | next utility |
| 8 | "Die Bank zahlt dir eine Dividende von 50 GE." | The bank pays 50 GE | — |
| 9 | "Mache einen Ausflug zum Fernbahnhof. Wenn du über Los kommst, ziehst du 200 GE ein." | Advance to the station; collect the salary if Los is passed | 5 |
| 10 | "Rücke vor bis zur Gartenstraße. Wenn du über Los kommst, ziehst du 200 GE ein." | Advance to the street; collect the salary if Los is passed | 11 |
| 11 | "Rücke vor bis zum Kontorplatz. Wenn du über Los kommst, ziehst du 200 GE ein." | Advance to the street; collect the salary if Los is passed | 24 |
| 12 | "Rücke vor bis zur Subprime-Allee." | Advance to the street; collect the salary if Los is passed (the printed text is silent about it) | 39 |
| 13 | "Dein Bausparvertrag wird fällig. Du erhältst 150 GE." | The bank pays 150 GE | — |
| 14 | "Strafe für zu schnelles Fahren. Zahle 15 GE." | Pay 15 GE to the bank | — |
| 15 | "Du bist zum Vorstand gewählt worden. Zahle jedem Spieler 50 GE." | Pay 50 GE to every other player (exception to the bank rule) | — |
| 16 | "Du lässt deine Häuser renovieren. Zahle 25 GE pro Haus und 100 GE pro Hotel." | Pay 25 GE per house and 100 GE per hotel to the bank | — |

**Gemeinschaftsfeld** (chest) — drawn at positions 2, 17, 33:

| # | Card text (German) | Effect | Target |
|---|--------------------|--------|--------|
| 1 | "Du kommst aus dem Gefängnis frei. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst." | Keep the card; use it to leave jail or sell it | — |
| 2 | "Gehe in das Gefängnis, begib dich direkt dorthin. Gehe nicht über Los. Ziehe nicht 200 GE ein." | Go to jail, no Los salary, turn ends | 10 |
| 3 | "Rücke vor bis auf Los, ziehe 200 GE ein." | Advance to Los, collect the salary | 0 |
| 4 | "Bank-Irrtum zu deinen Gunsten. Ziehe 200 GE ein." | The bank pays 200 GE | — |
| 5 | "Du erbst 100 GE." | The bank pays 100 GE | — |
| 6 | "Deine Lebensversicherung wird fällig. Du erhältst 100 GE." | The bank pays 100 GE | — |
| 7 | "Aus Lagerverkäufen erhältst du 50 GE." | The bank pays 50 GE | — |
| 8 | "Du erhältst eine Beratungsgebühr von 25 GE." | The bank pays 25 GE | — |
| 9 | "Einkommensteuerrückzahlung. Du erhältst 20 GE." | The bank pays 20 GE | — |
| 10 | "Zweiter Preis in einem Schönheitswettbewerb. Du erhältst 10 GE." | The bank pays 10 GE | — |
| 11 | "Du hast Geburtstag! Jeder Spieler schenkt dir 10 GE." | Every other player gives you 10 GE (exception to the bank rule) | — |
| 12 | "Urlaubsgeld! Du erhältst 100 GE." | The bank pays 100 GE | — |
| 13 | "Zahle Schulgeld. Zahle 50 GE." | Pay 50 GE to the bank | — |
| 14 | "Kosten für Straßenausbesserungen! Zahle 40 GE pro Haus und 115 GE pro Hotel." | Pay 40 GE per house and 115 GE per hotel to the bank | — |
| 15 | "Krankenhausgebühren. Zahle 100 GE." | Pay 100 GE to the bank | — |
| 16 | "Arztkosten. Zahle 50 GE." | Pay 50 GE to the bank | — |

#### 3.2.2 Card catalog format

The card catalog lives next to the board catalog (`backend/internal/board/`). Each card
record has:

- `id`: stable slug — survives text edits, referenced from logs and tests
- `deck`: `ereignisfeld` | `gemeinschaftsfeld`
- `text`: German display text (the table above)
- `effect`: the mechanical effect the engine executes — `advance_to`, `move_by`,
  `go_to_jail`, `get_out_of_jail`, `bank_payout`, `bank_payment`, `pay_each_player`,
  `collect_from_each_player`, `pay_per_house`, `next_station`, `next_utility`
- `target`: board position, or a relative offset, or empty — authoritative, never a
  street name
- `amount`: the GE amount for the money effects (may be a per-house/per-hotel rate)
- `losSalary`: whether passing Los pays the salary on this card — `true` for every
  forward-movement card, `false` for the backwards card and for the jail card

Loader validation: each deck holds exactly 16 cards, every `effect` is from the enum
above, every `target` is a valid board position in 0–39, and every money effect has a
positive `amount`. The loader refuses to start on an invalid catalog.

### 3.3 Entities

- **User** (in-memory): external id, display name, email, groups, first-seen timestamp.
- **Room**: id, short join code (6 letters), host user id, member list, phase
  (`lobby` | `playing` | `finished`), game state, created/last-activity timestamps.
- **Player** (in room): seat, user id, display name, token color, cash (never negative),
  position, in-jail flag, jail escape attempts, get-out-of-jail cards, owned tiles,
  houses/hotels built, mortgaged tiles, bankrupt flag, per-game statistics (rent
  collected/paid, properties bought, …).
- **Tile state** (per game): owner (player id or nil), house count (0–4, 5 = hotel),
  mortgaged flag.
- **Auction**: tile, current bid, current bidder, ends-at timestamp, bid history.
- **Trade offer**: proposer, receiver, offered/requested tiles and cash, status.
- **Bank**: the neutral party of the game — receives taxes, card payments, purchase
  prices, building costs and fines; pays out Los salary, card rewards, hotel/house
  refunds. No money limit.
- **Event log**: append-only list of game events (roll, move, payment, purchase, trade,
  card, jail, bankruptcy) with the acting player and affected amounts.

## 4. Features

### 4.1 Lobby

- A signed-in user creates a room; the server assigns a 6-letter join code. The create
  screen shows the code and a shareable link (`/room/<code>`).
- Other users join with the code or the link, pick a display name and a token color, and
  appear in the seat list immediately. A room holds 2–8 players.
- The lobby shows the seats with names and colors, the host's controls, and the rule set
  in effect (fixed for v1.0 — see 4.12).
- The host starts the game; the server assigns a **random seat order**, deals the
  starting cash, and the room switches to `playing`.
- A player may leave the lobby before the start; after the start, leaving means
  bankruptcy (4.10).

### 4.2 Turn flow

Each turn is a sequence of phases driven by the server:

1. **Pre-turn (open window)** — the active player may build houses/hotels (4.6) and
   propose or answer trades (4.8) *before* rolling. Other players may propose trades to
   each other at any time, but a trade involving the active player must be answered
   before that player rolls; an unanswered offer lapses at the roll.
2. **Roll** — the player rolls two dice (server-side, seeded from the system CSPRNG).
   Three doubles in a row send the player to jail; doubles grant another turn.
3. **Move and land** — the player moves and the landing tile is resolved (4.5, 4.7,
   4.8). A card may move the player again.
4. **Resolve pending decisions** — the server may wait for an explicit player decision
   (buy the tile, bid in an auction, choose a jail escape option). The game never blocks
   silently: after a timeout the game resolves the decision in the player's disfavour
   (declined purchase, no bid) and logs it.
5. **End turn** — the server advances to the next player. A player in jail who cannot or
   does not want to leave stays in jail and the turn passes.

The server is the only writer of the game state. Clients send intents; the server
validates them against the current phase and rejects illegal ones with an explicit error
the client shows. There are no silent fallbacks.

### 4.3 Money and the bank

- The bank is a neutral party with unlimited money. It receives taxes, card payments,
  purchase prices, building costs, and fines; it pays out Los salary, card rewards, and
  refunds for houses/hotels sold back.
- **Los salary**: 200 GE when a player passes or lands on Los. Landing on Los pays the
  salary only once — never double.
- **No Los salary** when a player is sent to jail (by the Zu-Gefängnis field or a card),
  and none when a player moves backwards (the "move back 3 tiles" card).
- **Taxes and card payments go to the bank**, not to a free-parking pot and not to
  another player. The Frei-Parken field therefore grants nothing.
- Rent goes to the property owner (minus the utility share, 4.5).

### 4.4 Buying property and auctions

- Landing on an unowned tile offers the purchase at the catalog price. The player may
  decline; a declined tile goes to auction.
- **Double-payment rule**: buying the tile that completes a color group costs **twice**
  the catalog price. The extra amount is paid to the bank. The rule applies to purchases
  from the bank, not to trades (4.8).
- **Auction**: all players (including the one who declined) may bid. Bids must be
  divisible by 20 and must exceed the current bid; the minimum first bid is the catalog
  price. The auction ends when no higher bid arrives within the countdown.
- **Auction completion rule**: if the winning tile completes a color group, the winner
  pays **120 % of its bid** instead of the bid.
- A tile nobody bids on stays with the bank and can be bought at the catalog price
  (including the double-price rule) by the next player who lands on it.
- The auction countdown is visible to all; a bid in the last seconds extends the auction
  by a short interval (anti-sniping).

### 4.5 Rent

- Rent for a street depends on the number of houses/hotels built on it.
- **Doubled unbuilt rent**: if the owner holds the complete color group and the street is
  still unbuilt, the rent is **twice** the base rent (classic rule, "Miete bei ganzer
  Farbgruppe").
- A mortgaged property yields no rent.
- **Jail rule**: while a property owner sits in jail, that owner collects **no rent**.
  The tenant does not pay anything — no rent is due, no debt is created, nothing happens.
- Utilities: rent is 4× the dice total; if one player owns both utilities it is 10×.
- **Utility share (Subprime rule)** — part of every rent flows to the utilities:
  - The owner of a **rosa** or **orange** street collects the full rent from the tenant
    and then pays **10 %** of the collected rent to the owner of the **Elektrizitätswerk**.
  - The owner of a **rot** or **gelb** street collects the full rent and then pays
    **10 %** (rounded down) of the collected rent to the owner of the **Wasserwerk**.
  - The share is never paid to the property owner themselves: if the property owner also
    owns the utility, no transfer happens.
  - If the utility tile is unowned, the share goes to the bank.
  - **Rounding**: a fractional share is rounded down when both parties are players, and
    rounded up (against the property owner) when the receiving side is the bank — the
    general Subprime rule "amounts that cannot be paid out are always rounded against
    the player; if only players are affected, round down".

### 4.6 Building

- A player who owns a complete color group may build houses on its streets and a hotel
  on the first street, as long as the bank has the building material.
- Houses cost the catalog build cost. A **hotel counts as five houses**: building it
  costs the build cost once (as for a house) and the four houses standing on the
  property are returned to the bank.
- Selling a hotel back refunds `floor(buildCost / 2) × 5` GE — half the build price for
  each of the five houses it represents — and returns one hotel plus four houses to the
  bank.
- Building is even within a color group (all streets of the group at the same level); a
  hotel counts as five houses for that comparison.
- The bank has a finite stock of building material (32 houses, 12 hotels) — the classic
  supply rule.
- Building happens in the pre-turn window (4.2), never while a move is pending.
- Houses/hotels can be sold back to the bank at half the build price each.

### 4.7 Property, mortgages, and selling

- **Mortgage**: the bank lends the owner **50 %** of the purchase price. A mortgaged
  property yields no rent and cannot be built on. To take the property back the owner
  pays **60 %** of the purchase price (50 % principal plus 10 % interest).
- **Selling to the bank**: the bank buys a property for **70 %** of its purchase price.
  Houses/hotels on it must be sold back first.
- Both mechanisms are available (the German board has no mortgages, the international one
  has no selling to the bank — Subprime has both).
- A mortgaged or sold property returns to the bank and can be bought again from the bank
  at the catalog price (with the double-price rule if it completes a color group).

### 4.8 Trading

- Trading is allowed and happens **before rolling** (Subprime rule): the active player
  may trade in their pre-turn window; other players may trade with each other at any
  time.
- A player proposes a trade (tiles and/or cash in both directions); the other player
  accepts, declines, or counter-proposes. Accepted trades are applied by the server
  atomically (ownership transfer plus cash transfer).
- **Only unbuilt properties can be traded.** A property with houses or a hotel must have
  its buildings sold back to the bank first (classic rule).
- The **get-out-of-jail card is tradable** like a property: it is offered in the trade
  engine alongside street cards and sold for cash.
- **Krumme Deals are forbidden but not judged in v1.0.** The game trusts the players to
  follow the rules. What is forbidden: giving away assets without consideration,
  conditional obligations ("if I land there, then…", future discounts), and trades whose
  expected value is obviously negative for one party.
- The game offers no assistance in judging a trade and never warns about one. Later
  versions may add an AI decider that flags suspect trades (section 8).
- **No debt**: a trade may only involve cash that both players actually have. The server
  rejects a trade whose cash leg either player cannot cover. A player who cannot pay a
  rent, tax, card payment, or fine goes bankrupt immediately (4.10).

### 4.9 Cards

- Landing on an `Ereignisfeld` or `Gemeinschaftsfeld` draws a card from the matching deck;
  the drawn card is shown to all players, then applied:
  - movement (advance to a field, advance to Los, go to jail, get out of jail free)
  - bank payout (the bank pays the amount) or bank payment (the player pays the bank)
  - the two player-to-player cards keep their classic behaviour (3.2)
  - the two cards with their own rent rule charge double rent / 10× dice (3.2)
- A card that moves the player resolves the destination tile as a normal landing.
- **Chaining is unlimited**: if a card moves the player onto a chance/chest field, the
  next card is drawn and applied as well, and so on. (In the German deck no card moves
  the player onto a card field, so a chain only starts via the "move back three tiles"
  card — and it always terminates, because no card sends the player back onto a card
  field.)
- The card effect is applied by the server; the UI animates it.

### 4.10 Jail, bankruptcy, end of game

- **Jail**: three failed escape attempts, or a card, send a player to jail. On the third
  attempt the player must pay the jail fine (50 GE). Rolling a double while in jail
  leaves jail; otherwise the player stays and the turn passes.
- **Bankruptcy**: a player who cannot cover a payment becomes bankrupt. The liquidation
  runs in this order:
  1. Houses and hotels are sold back to the bank at half the build price.
  2. The remaining properties are **sold to the bank** at the sell-back price (70 % of
     the purchase price, 4.7); mortgaged properties go back to the bank directly and pay
     nothing.
  3. All money — the player's cash plus the proceeds from steps 1 and 2 — goes to the
     **creditor**. With no creditor (a tax, a card payment, a fine) it goes to the bank.
  4. Get-out-of-jail cards go to the creditor, or back under their deck if there is none.
  5. Any open trades involving the player are cancelled and they are removed from any
     running auction.
- A bankrupt player leaves the game; their seat stays visible as eliminated and their
  turns are skipped.
- **End of game**: the game ends when only one player is left. The final screen shows the
  standings: net worth (cash + property value), rent collected/paid, and the winner.
- Disconnecting does not end the game: a disconnected player keeps their seat and the
  turn waits for them (with the timeout rule from 4.2). Reconnecting with the same user
  id restores the seat.

### 4.11 Spectators and the event log

- Users who open a running room without a seat watch as spectators and see the full
  board and the event log. **A spectator stays a spectator for the whole game** — they
  cannot take a seat later, not even after another player went bankrupt.
- The event log records every game event in order, with amounts and the acting player;
  it is the authoritative history of the game and is shown in a scrollable panel.
- There is **no chat** — not in v1.0 and not later. The players talk over a voice call;
  the event log is the record.

### 4.12 Rule set for v1.0

The following are fixed for v1.0 and shown in the lobby:

| Rule | Value |
|------|-------|
| Players | 2–8 per room |
| Starting cash | 1500 GE |
| Seat order | random, decided by the server |
| Los salary | 200 GE, no double on landing |
| Taxes and card payments | to the bank (except the two classic player-to-player cards) |
| Card decks | Ereignisfeld + Gemeinschaftsfeld, 16 cards each, card targets resolved by position |
| Station / utility cards | next station = double rent, next utility = 10× dice |
| Auction | declined tile goes to auction, bids in multiples of 20 |
| Color group completion | double price on purchase, 120 % of bid in auction |
| Rent in jail | owner collects no rent, tenant pays nothing |
| Unbuilt rent with a complete color group | doubled (classic rule) |
| Utility rent | 4× dice, 10× dice when both utilities are owned |
| Utility share | 10 % of rosa/orange rent to Elektrizitätswerk, 10 % (down) of rot/gelb rent to Wasserwerk |
| Mortgage | bank lends 50 %, unmortgage costs 60 % |
| Selling to the bank | 70 % of the purchase price |
| Buildings | even within a group, bank stock 32 houses / 12 hotels |
| Trading | allowed before rolling, unbuilt properties only, not judged |
| Debt | none — a trade may only use cash both players actually have |
| Bankruptcy | buildings sold back at half, properties sold to the bank, money to the creditor |
| Hotel | counts as five houses; costs the build cost once, four houses return to the bank |
| Card chaining | unlimited |
| Krumme Deals | forbidden by rule, not enforced in v1.0 |

## 5. AI

None in v1.0. The Krumme-Deals judging is explicitly postponed; the game engine keeps
the trade data (offered tiles, cash, expected-value inputs) in a shape that a later AI
decider can consume, but v1.0 makes no AI calls.

## 6. Technology Stack

| Layer | Choice | Status |
|-------|--------|--------|
| Frontend | Svelte 5 / SvelteKit as static SPA, Material Web Components | decided |
| Design system | Corporate design tokens (Material 3), light default, dark toggle | decided |
| Backend | Go, single binary, HTTP + WebSocket | decided |
| Realtime | WebSocket (server pushes state, clients send intents) | decided |
| State | In memory only, no database | decided |
| Auth | OAuth2 proxy headers, no local login | decided |
| Deployment | Docker (compose), behind Timo's reverse proxy | decided |

### Backend language reasoning

The backend is an authoritative game server: a small REST surface for room
creation/joining and a WebSocket channel for live play. It holds all state in memory,
does dice rolls and money arithmetic, and broadcasts state changes.

- **Go**: one static binary, tiny container image, negligible memory footprint,
  goroutines fit the "one goroutine per connected client, one mutex per room" model of
  an in-memory game server, and the stdlib HTTP server needs no framework. Strong typing
  keeps a large rules engine (rents, auctions, trades, rounding) honest.
- **TypeScript/SvelteKit server routes**: one language for the whole app, but ties the
  game server to the Node runtime and to a framework whose request model does not fit
  long-lived WebSocket connections as well.
- **Python/FastAPI**: comfortable for rule scripting, but heavier runtime image, slower
  startup, and async WebSocket handling adds no value here.
- The downside of Go is more boilerplate than TypeScript; at this size that is acceptable
  and partly offset by the absence of a runtime and of a database.

### Frontend

- SvelteKit in static SPA mode (`adapter-static`, `fallback: 'index.html'`), built assets
  copied into the container image at `/app/static` and served **from disk** by the Go
  backend via the `STATIC_DIR` environment variable — the same approach as coach-leo.
  No separate frontend container in production.
- The Go module path is `github.com/DT-Entertainment-Studios/subprime-game/backend`.
- UI follows the corporate design: Material 3 tokens (`@material/web` components, icons
  only from `@tabler/icons-svelte`), light theme by default, dark theme as a toggle,
  DejaVu Sans from the token typography.
- Realtime client: native WebSocket with a small reconnecting wrapper; all game intents
  go over the same socket.
- Entire UI in German; no i18n framework in v1.0.

### API shape

- REST for non-realtime operations: `POST /api/rooms` (create),
  `POST /api/rooms/{code}/join`, `GET /api/rooms/{code}`, `GET /api/me`,
  `GET /api/health`.
- WebSocket at `/api/ws?room=<code>` for everything that happens during play: the server
  sends the full game state after every change (small state, no diffing needed) plus
  discrete events for animations; the client sends intents (`roll`, `buy`, `decline`,
  `bid`, `build`, `sell_building`, `mortgage`, `unmortgage`, `sell_to_bank`,
  `propose_trade`, `respond_trade`, `jail_option`, `end_turn`).
- The message contract is defined once in Go structs and mirrored by TypeScript types in
  the frontend; no hand-written API schema file.
- **Response shape**: successful responses are plain domain objects (no envelope), e.g.
  `{"room":{"code":"ABCDEF", …}}`. **Errors** always use one shape:
  `{"error":{"message":"…","code":"…"}}` with an optional machine-readable `code`.
- **Status codes**: 401 missing or blank identity header · 403 not permitted ·
  404 unknown room (also for rooms the caller may not see, so existence does not leak) ·
  405 known path, wrong method (with the `Allow` header) · 400 invalid request body ·
  500 internal error (generic message; details only in the log) · 200 for health.

### Testing

- **Backend**: Go standard library only — no testify, no go-cmp. Table-driven tests with
  the loop variable named `tc` and the slice named `tests`; assertions use plain
  `t.Errorf("status = %d, want %d", …)`. HTTP tests use `net/http/httptest` and call the
  handler in-process (no network). Fakes are hand-written structs in the test file.
  Identity headers are set explicitly per request.
- **The rules engine is the test target**: the board catalog, rent ladders, the doubled
  unbuilt rent, the utility share and its rounding, the double-price and 120 %-of-bid
  rules, mortgage/sell arithmetic, auction increments, card effects, and the bankruptcy
  liquidation are all covered by table-driven unit tests over the engine.
- **Frontend**: Vitest with jsdom, `@testing-library/svelte` and `@testing-library/jest-dom`.
  Test files sit next to the code (`src/**/*.test.ts`), shared fixtures live in one
  testing module, and `fetch` is stubbed globally.

### Realtime protocol choice

WebSocket over SSE: the client must both receive pushes and send intents over the same
long-lived connection; SSE would need a second HTTP channel for commands and cannot
carry the low-latency auction bidding that the game needs.

### Configuration

Environment variables only, no command-line flags — one source of truth, no precedence
rules to get wrong. Every value is trimmed; an empty or whitespace-only value counts as
unset and falls back to the default. Configuration problems are collected and reported
together in one startup error instead of one per attempt.

| Variable | Required | Default | Meaning |
|----------|----------|---------|---------|
| `PORT` | no | `8080` | Backend listen port |
| `STATIC_DIR` | no | `""` | Directory with the built SPA; empty ⇒ API only |
| `AUTH_HEADER_USER` | no | `X-Forwarded-User` | Header carrying the user id (the auth gate) |
| `AUTH_HEADER_NAME` | no | `X-Forwarded-Preferred-Username` | Header carrying the display name |
| `AUTH_HEADER_EMAIL` | no | `X-Forwarded-Email` | Header carrying the e-mail address |
| `AUTH_HEADER_GROUPS` | no | `X-Forwarded-Groups` | Header carrying the group list |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |

`STATIC_DIR`, when set, must exist at startup — a typo would otherwise surface only as
the SPA fallback answering every request, so it fails fast.

## 7. Non-Goals (first version)

- **Krumme-Deals enforcement or AI judging** — postponed; the players are trusted.
- **Chat or any in-app communication feature** — never; the players use a voice call.
- Own login, registration, or password handling — the OAuth2 proxy owns authentication.
- Any database or persistence across server restarts — a restart ends all games.
- Cross-game profiles, statistics, or leaderboards — user preferences live in the
  browser only.
- User-editable boards, a board editor, or multiple board layouts — one fixed board.
- Configurable house rules in the lobby — the rule set is fixed for v1.0 (4.12).
- Bot/AI players.
- Mobile native apps — responsive web only.
- Languages other than German.
- Spectators joining the game as a player.

## 8. Later Versions

- **AI decider for Krumme Deals**: an AI that evaluates a proposed trade and flags it
  when it matches one of the forbidden patterns (gift without consideration, future
  obligations, obviously negative expected value). The engine keeps the trade payload in
  a form the decider can consume.
- Configurable house rules in the lobby (starting cash, auction on/off, even building,
  jail fine, double on Los).
- More than one board layout.
- Persistence of finished games and statistics.
- Public matchmaker and random opponents.

## 9. Decision Log

Why each open point was resolved the way it was.

- **Q1 — Street names**: renamed to neutral, thematically fitting names
  (Zollgasse, Bankstraße, Börsenplatz, Subprime-Allee …) instead of the classic German
  names, so the repository stays safe to publish. The color group names stay German
  because the rules refer to them (`rosa`, `orange`, `rot`, `gelb`).
- **Q2 — Bank pool / draw rule**: the wiki rule "if the money in the bank is empty the
  game is judged a draw (20 000 GE are in the game)" was **removed** — it is not a good
  rule. The bank has unlimited money; the game always ends with a winner. (This
  deviates from the wiki page; the wiki entry should be updated.)
- **Q3 — Utility share**: the property owner collects 100 % of the rent and then pays
  10 % of the collected rent to the utility owner (option 2). The tenant pays only the
  rent.
- **Q4 — Trading**: an in-game trade flow exists in v1.0 (tiles and cash both ways,
  accepted by the other player, before rolling) with **no judging and no warnings**.
- **Q5 — Multiplayer**: online rooms. A host creates a room, others join with a
  6-letter code or a shareable link, 2–8 players plus spectators, real-time over
  WebSocket.
- **Q6 — Jail rent**: the tenant pays nothing — no rent is due, no debt is created.
- **Q7 — Chat**: no chat, not in v1.0 and not later.
- **Q8 — Spectator becoming a player**: not possible; spectators stay spectators.
- **Players and starting cash**: 2–8 players, 1500 GE starting cash each (classic).
- **Mortgage and selling**: both mechanisms exist. The bank lends 50 % of the purchase
  price; unmortgaging costs 60 %; selling to the bank yields 70 %.
- **Trading buildings**: only unbuilt properties can be traded; buildings must be sold
  back to the bank first.
- **Starting player**: the server assigns a random seat order; no opening roll.
- **Player-to-player cards**: the two classic cards that move money between players
  ("Geburtstag" collects 10 GE from every player, "Vorstand" pays 50 GE to every player)
  keep their classic behaviour. The bank rule covers card *payments*, not card
  *collections*.
- **Deck field names**: renamed to **Ereignisfeld** / **Gemeinschaftsfeld** — the names
  printed on the physical German board, and generic German words consistent with the
  renamed streets. (The physical decks are called Ereigniskarten / Gemeinschaftskarten.)
- **Card texts naming streets**: rewritten to use the renamed streets and stored as data
  next to the card catalog, so a street rename stays a one-place change. Card targets are
  resolved by position, never by name.
- **Cards with their own rent rule**: kept as in the classic rules — the "next station"
  card charges double rent, the "next utility" card charges 10× the dice total.
- **Get-out-of-jail card**: tradable like a property, offered in the trade engine
  alongside street cards.
- **Doubled unbuilt rent**: added after the board verification against the official
  German rulebook — if the owner holds the complete color group, an unbuilt street
  charges twice its base rent (classic rule, "Miete bei ganzer Farbgruppe"). It was
  missing from the first draft.
- **Tax field names**: corrected to the names on the official German board —
  *Einkommensteuer* (200 GE) and *Zusatzsteuer* (100 GE).
- **Board data verified**: all 40 fields (order, color groups, prices, rent ladders, build
  costs, mortgage values) were checked against the official German rulebook and the
  German board artwork and match the table in section 3.1 exactly.
- **Los salary edge cases**: no salary when a player is sent to jail (field or card) and
  none when moving backwards.
- **First color group** is `braun` (post-2008 / international printings); the pre-2008
  classic German board printed it violet. The groups the rules name are unaffected.
- **SPA serving**: the built frontend is copied into the image at `/app/static` and served
  from disk via `STATIC_DIR` (as in coach-leo), not embedded into the binary with
  `go:embed` as the wiki recipe describes. The working sibling wins over the wiki here.
- **Module path**: `github.com/DT-Entertainment-Studios/subprime-game/backend`, matching
  the current Readme.
- **Frontend dev port**: 8000, per the wiki guidelines for new projects (coach-leo runs
  5173).
- **Configuration**: environment variables only, no CLI flags — one source of truth, as in
  coach-leo. The variable table is in section 6.
- **Currency in the card texts**: the German cards print `M` (Monopoly Money); Subprime
  uses `GE`, so the printed amounts were converted. Card targets are stored as board
  positions, never as street names — the classic German board rotates the station names
  and repeats a street name, so name-based lookup would resolve to the wrong tile.
- **Card catalog format** (3.2.2): every card carries `id`, `deck`, `text`, `effect`,
  `target`, `amount`, and `losSalary`; the loader validates 16 cards per deck and refuses
  to start on an invalid catalog.
- **No debt**: a trade may only use cash both players actually have. Any payment a player
  cannot cover ends in bankruptcy. (The earlier draft allowed trades to push a player into
  negative cash — dropped, it added a settlement model without a purpose.)
- **Bankruptcy liquidation** (4.10): buildings are sold back at half the build price, the
  remaining properties are sold to the bank (mortgaged ones return directly), and all the
  money goes to the creditor. Get-out-of-jail cards go to the creditor too.
- **Card chaining**: unlimited. In the German deck no card moves the player onto a card
  field, so chaining only starts via the "move back three tiles" card and always
  terminates.
- **Hotel**: counts as five houses — building it costs the build cost once and the four
  houses return to the bank; selling it back refunds half the build price per house it
  represents.

## 10. Remaining Assumptions (veto-able, no decision needed if accepted)

- **Spectators** may watch any room, also a finished one.
- **Turn timeout**: a pending player decision resolves after 60 s in the player's
  disfavour (declined purchase, no bid) — configurable via a flag, not exposed in the UI.
- **Connection loss** keeps the seat; the turn waits for the player (timeout rule
  applies).
- **Auction**: countdown 15 s, extended by 10 s on a bid in the last 5 s; minimum first
  bid is the catalog price; bids in multiples of 20.
- **Bank stock** of building material: 32 houses, 12 hotels (classic supply).
- **Cards**: 16 + 16 cards, classic German decks, adapted so that all card payments go to
  the bank and the two player-to-player cards are exempt (3.2, 4.9).
- **Card register**: the German decks exist in an informal ("du") and a formal ("Sie")
  variant with identical effects. v1.0 uses the informal variant consistently.
- **Two card details are unverified against the sources** and are marked as such in the
  card catalog: the exact wording of the second "advance to the next station" card, and
  whether the M50 dividend card is still part of the current German chance deck. Both
  have a documented fallback (see the card research notes).
- **Utility share rounding**: rounded down when both parties are players, rounded up
  when the share goes to the bank.
- **Jail fine**: 50 GE; three failed escape attempts, then the fine is mandatory.
- **Room codes**: 6 uppercase letters, unambiguous alphabet (no O/0, I/1).
- **Language**: UI and all game content in German; repository, spec, code, and comments
  in English.
- **Ports**: backend 8080, frontend dev server 8000.
- **Deployment**: Docker compose behind Timo's reverse proxy, oauth2-proxy as the only
  ingress; production compose stays outside the repository, only `docker-compose-dev.yaml`
  is committed. The dev stack mocks the proxy with a Caddy service that injects the
  identity headers, as in coach-leo.
- **Container images**: the runtime image is `madebytimo/base`, with `madebytimo/nodejs`
  for the frontend build and `madebytimo/go` for the backend build. A Go healthcheck
  binary is compiled in, since the runtime image has no shell.