# Subprime — Specification

Status: v1.0 draft · all open questions resolved (see section 9, Decision Log) ·
remaining veto-able assumptions in section 10.

## 1. Overview

Subprime is a self-hosted online multiplayer board game for a group of friends. It is a
Monopoly-like game with the classic 40-field board and the additional "Subprime" house
rules: buying the last street of a color group costs double, part of the rent of the
rosa, orange, rot and gelb streets flows to the utilities, and crooked deals ("Krumme
Deals") are forbidden. One player creates a
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
- Display name and token color are chosen by the user.

## 3. Domain Model

All monetary amounts are integers in GE ("Geld-Einheiten"), the unit used in the rules.

### 3.1 Board catalog

The board is the classic 40-field board with the classic German color groups. The catalog
is the single source of truth for the game and lives in the backend
(`backend/internal/board/`): a static, hand-maintained Go source file in the repository,
not user-editable at runtime and not loaded from an external file at startup.

Tile types: `go`, `street`, `tankstelle`, `utility`, `tax`, `chance`, `chest`, `jail`,
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
| 4 | Einkommensteuer | tax | — | — | 10 % of cash | — | — |
| 5 | Tankstelle Süd | tankstelle | tankstelle | 200 | 25, 50, 100, 200 | — | 140 |
| 6 | Marktstraße | street | hellblau | 100 | 6, 30, 90, 270, 400, 550 | 50 | 70 |
| 7 | Ereignisfeld | chance | — | — | — | — | — |
| 8 | Handelsweg | street | hellblau | 100 | 6, 30, 90, 270, 400, 550 | 50 | 70 |
| 9 | Uferpromenade | street | hellblau | 120 | 8, 40, 100, 300, 450, 600 | 50 | 84 |
| 10 | Werkstatt | jail | — | — | — | — | — |
| 11 | Gartenstraße | street | rosa | 140 | 10, 50, 150, 450, 625, 750 | 100 | 98 |
| 12 | Elektrizitätswerk | utility | versorger | 150 | 4× dice, 10× dice with both | — | 105 |
| 13 | Lindenplatz | street | rosa | 140 | 10, 50, 150, 450, 625, 750 | 100 | 98 |
| 14 | Rosengarten | street | rosa | 160 | 12, 60, 180, 500, 700, 900 | 100 | 112 |
| 15 | Tankstelle West | tankstelle | tankstelle | 200 | 25, 50, 100, 200 | — | 140 |
| 16 | Ahornring | street | orange | 180 | 14, 70, 200, 550, 750, 950 | 100 | 126 |
| 17 | Gemeinschaftsfeld | chest | — | — | — | — | — |
| 18 | Eichenweg | street | orange | 180 | 14, 70, 200, 550, 750, 950 | 100 | 126 |
| 19 | Ulmenplatz | street | orange | 200 | 16, 80, 220, 600, 800, 1000 | 100 | 140 |
| 20 | Frei-Parken | parking | — | — | — | — | — |
| 21 | Bankstraße | street | rot | 220 | 18, 90, 250, 700, 875, 1050 | 150 | 154 |
| 22 | Ereignisfeld | chance | — | — | — | — | — |
| 23 | Zinsgasse | street | rot | 220 | 18, 90, 250, 700, 875, 1050 | 150 | 154 |
| 24 | Kontorplatz | street | rot | 240 | 20, 100, 300, 750, 925, 1100 | 150 | 168 |
| 25 | Tankstelle Nord | tankstelle | tankstelle | 200 | 25, 50, 100, 200 | — | 140 |
| 26 | Anleiheweg | street | gelb | 260 | 22, 110, 330, 800, 975, 1150 | 150 | 182 |
| 27 | Renditenstraße | street | gelb | 260 | 22, 110, 330, 800, 975, 1150 | 150 | 182 |
| 28 | Wasserwerk | utility | versorger | 150 | 4× dice, 10× dice with both | — | 105 |
| 29 | Dividendenring | street | gelb | 280 | 24, 120, 360, 850, 1025, 1200 | 150 | 196 |
| 30 | Unfall | gotojail | — | — | — | — | — |
| 31 | Börsenplatz | street | grün | 300 | 26, 130, 390, 900, 1100, 1275 | 200 | 210 |
| 32 | Depotstraße | street | grün | 300 | 26, 130, 390, 900, 1100, 1275 | 200 | 210 |
| 33 | Gemeinschaftsfeld | chest | — | — | — | — | — |
| 34 | Fondsplatz | street | grün | 320 | 28, 150, 450, 1000, 1200, 1400 | 200 | 224 |
| 35 | Tankstelle Ost | tankstelle | tankstelle | 200 | 25, 50, 100, 200 | — | 140 |
| 36 | Ereignisfeld | chance | — | — | — | — | — |
| 37 | Kreditstraße | street | dunkelblau | 350 | 35, 175, 500, 1100, 1300, 1500 | 200 | 245 |
| 38 | Zusatzsteuer | tax | — | — | 100 | — | — |
| 39 | Subprime-Allee | street | dunkelblau | 400 | 50, 200, 600, 1400, 1700, 2000 | 200 | 280 |

Group sizes: braun 2, hellblau 3, rosa 3, orange 3, rot 3, gelb 3, grün 3, dunkelblau 2,
tankstelle 4, versorger 2.

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
  - "advance to the next Tankstelle": if owned, pay the owner **double** the normal rent; if
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
| 2 | "Du hast einen Unfall: Begib dich direkt in die Werkstatt. Gehe nicht über Los. Ziehe nicht 200 GE ein." | Go to jail, no Los salary, turn ends | 10 |
| 3 | "Pannenhilfe: Du kommst gratis aus der Werkstatt. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst." | Keep the card; use it to leave jail or sell it | — |
| 4 | "Gehe drei Felder zurück." | Move back 3 tiles and resolve that tile (no Los salary) | −3 |
| 5 | "Rücke vor bis zur nächsten Tankstelle. Wenn die Tankstelle noch niemandem gehört, kannst du sie von der Bank kaufen. Falls sie einen Besitzer hat, zahle ihm das Doppelte der normalen Miete." | Advance to the next Tankstelle; buy it if unowned, otherwise pay double rent | next Tankstelle |
| 6 | "Rücke vor bis zur nächsten Tankstelle. Gehört die Tankstelle niemandem, kannst du sie von der Bank kaufen. Hat sie einen Besitzer, zahlst du ihm das Doppelte der normalen Miete." | Same as card 5 — the second nearest-Tankstelle card | next Tankstelle |
| 7 | "Rücke vor bis zum nächsten Versorgungswerk. Gehört das Werk niemandem, kannst du es von der Bank kaufen. Sonst wirfst du die Würfel und zahlst dem Eigentümer das Zehnfache deines Würfelergebnisses." | Advance to the next utility; buy it if unowned, otherwise pay 10× dice | next utility |
| 8 | "Die Bank zahlt dir eine Dividende von 50 GE." | The bank pays 50 GE | — |
| 9 | "Mache einen Ausflug zur Tankstelle Süd. Wenn du über Los kommst, ziehst du 200 GE ein." | Advance to the Tankstelle; collect the salary if Los is passed | 5 |
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
| 1 | "Pannenhilfe: Du kommst gratis aus der Werkstatt. Diese Karte musst du behalten, bis du sie benötigst oder verkaufst." | Keep the card; use it to leave jail or sell it | — |
| 2 | "Du hast einen Unfall: Begib dich direkt in die Werkstatt. Gehe nicht über Los. Ziehe nicht 200 GE ein." | Go to jail, no Los salary, turn ends | 10 |
| 3 | "Rücke vor bis auf Los, ziehe 200 GE ein." | Advance to Los, collect the salary | 0 |
| 4 | "Bank-Irrtum zu deinen Gunsten. Ziehe 200 GE ein." | The bank pays 200 GE | — |
| 5 | "Du erbst 100 GE." | The bank pays 100 GE | — |
| 6 | "Deine Lebensversicherung wird fällig. Du erhältst 100 GE." | The bank pays 100 GE | — |
| 7 | "Aus Lagerverkäufen erhältst du 50 GE." | The bank pays 50 GE | — |
| 8 | "Du erhältst eine Beratungsgebühr von 25 GE." | The bank pays 25 GE | — |
| 9 | "Einkommensteuerrückzahlung. Du erhältst 20 GE." | The bank pays 20 GE | — |
| 10 | "Du hast in einem Drag Race gewonnen. Du erhältst 10 GE." | The bank pays 10 GE | — |
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
  `collect_from_each_player`, `pay_per_house`, `next_tankstelle`, `next_utility`
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
- **Presence** (in-memory): per user the set of open start-page connections,
  deduplicated; this is the raw presence set, and a user is in it while at least one
  connection is open. The lobby online list additionally excludes every user who is
  currently in a room (strict D2): a seat plus either a live room connection or a game
  that has not finished. A seat left in a finished room does not count. Presence is not
  part of the game state.
- **Invitation** (in-memory): id, room code, inviter id and display name, invitee id,
  created-at timestamp. At most one pending invitation exists per room and invitee.
- **Player** (in room): seat, user id, display name, token color, cash (may be negative
  while the player settles a debt), position, in-jail flag, jail escape attempts,
  get-out-of-jail cards, owned tiles, houses/hotels built, mortgaged tiles, bankrupt
  flag, per-game statistics (rent collected/paid, properties bought, …).
- **Tile state** (per game): owner (player id or nil), house count (0–4, 5 = hotel),
  mortgaged flag.
- **Auction**: tile, current bid, current bidder, ends-at timestamp, bid history.
- **Trade offer**: proposer, receiver, offered/requested tiles, cash and workshop
  passes, status (open · accepted · declined · lapsed · countered) A counter-proposal
  replaces the offer it references).
- **Bank**: the neutral party of the game — receives taxes, card payments, purchase
  prices, building costs and fines; pays out Los salary, card rewards, hotel/house
  refunds. No money limit.
- **Event log**: append-only list of game events (roll, move, payment, purchase, trade,
  card, jail, debt, give-up, bankruptcy) with the acting player and affected amounts.

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
- **"Verlassen" in an open lobby** frees the seat through
  `POST /api/rooms/{code}/leave` (204 No Content, no body); the client then closes its
  room connection and the room-presence subscription and returns to the start page.
  The start-page presence reconnects there. If the host leaves, the host role
  passes to the next member in seat order. A link-only visitor (spectator) also returns
  to the start page; the request is a no-op for them. A leave that fails for any reason
  other than an already-gone room keeps the seat and the user on the room page instead
  of dropping the seat silently. The brand link in the topbar follows the same path in
  an open lobby — a plain left click triggers the leave request and the return to `/` —
  while in `playing` and `finished` it stays a plain link to the start page. Leaving a
  running or finished game only closes the view (4.10).
- **Start-page banner for the caller's own room**: while the caller counts as "in a room"
  (the rule below), the start page shows that room's code and phase (`Lobby`,
  `Spiel läuft` or `Spiel beendet`) instead of leaving the state implicit. The query
  `GET /api/me/room` answers `{"room":null}` or
  `{"room":{"code":"…","phase":"lobby|playing|finished"}}` and uses the same rule as
  create, join and invitations. "Wieder beitreten" navigates to `/room/<code>`, where the
  existing load/reconnect flow restores the seat. "Raum verlassen" asks for confirmation
  and calls `POST /api/rooms/{code}/abandon` (204): in a lobby that is the normal leave
  including host promotion and last-member close, in a running game an immediate forfeit
  plus removal of the membership (4.10); a seat that is already eliminated only loses its
  stale membership (no second elimination, no asset move, no turn change), in a finished
  game the removal of the stale seat. Creating and joining are disabled while the banner shows and enabled again after
  a successful abandon; the banner has no dismiss action, and the query refreshes on
  mount and whenever the window regains focus or becomes visible again. A 404
  `room_not_found` (the room is already gone) counts as success.
- When the last member leaves, the lobby is **closed**: its code and link answer 404
  `room_not_found` from then on — for `GET /api/rooms/{code}`, the join endpoint, the
  room WebSocket and the presence WebSocket — and its pending invitations die with it.
- `seats` is always a JSON array (`[]` in an empty lobby), never `null`.
- The lobby shows a live list of the signed-in users currently waiting on the start
  page ("In der Lobby online"): start-page connections only, deduplicated per user,
  without any user who is currently in a room, each with the display name last
  confirmed on the start page (the identity name until then). "In a room" means a
  seat plus either a live room connection or a game that has not finished; a seat left
  in a finished room does not count. A seat in a live room — including a paused seat
  in a running game — blocks creating or joining a different room with 403
  `already_in_room` naming that room's code. Rejoining the same room is always allowed,
  and so is joining a finished game while no room connection to it is open (the "in a
  room" definition in 3.3).
- Every room member may invite an online user into the room. An invited user is marked
  "eingeladen" and appears in the sent-invitations list with an `Abbrechen` action for
  the member who sent it; a rejected action (target offline, duplicate invitation, room
  no longer open) shows a dismissible error in the lobby.
- An invitee sees every open invitation on the start page as a persistent panel, plus
  a transient notice for a newly arriving invitation. Accepting joins the room directly
  with the name currently entered on the start page (confirmed or not, 2) and the
  token color and opens `/room/<code>`; declining removes the invitation.
- An invitation ends on accept, decline, cancellation by the inviter, room start, room
  full, room closed or the invitee joining the room by code — there is no time limit.
- An accept rejected for an invalid colour or because the invitee is still in another
  room keeps the invitation, so it can be retried; room-state failures remove it.
- The name shown in the lobby's online list is published when the user confirms it
  (`Namen übernehmen`/Enter) and re-sent after a reconnect when it differs from the
  login name, so the online list and the invitee's name in the sent-invitations list
  follow a confirmed rename without a reload; the inviter name in an existing
  invitation is fixed at creation.

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
   (buy the tile, bid in an auction, choose a jail escape option). Except the auction nothing has a time limit.
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
- **No Los salary** when a player is sent to jail (by the Unfall field or a card),
  and none when a player moves backwards.
- **Taxes and card payments go to the bank**, not to a free-parking pot and not to
  another player. The Frei-Parken field therefore grants nothing. A car that lands
  there only pulls into one of the field's marked parking bays (purely visual
  detail).
- **Tax fields**: *Einkommensteuer* (field 4) charges **10 % of the player's cash at
  the moment of landing**, rounded down to whole GE. *Zusatzsteuer* (field 38) keeps
  its fixed 100 GE. The percentage is computed from cash only (not net worth) and
  before any liquidation that the payment might trigger.
- Rent goes to the property owner (minus the utility share, 4.5).

### 4.4 Buying property and auctions

- Landing on an unowned tile offers the purchase at the catalog price. The player may
  decline; a declined tile goes to auction. A player who cannot cover the price yet still
  gets the offer and can bid unlimited. They get into debt as if they had to pay something when they have not enough money to fullfil their bid.
- The tile only goes to auction when the player
  actively starts the auction. It is never auctioned automatically.
- **Double-payment rule**: buying the tile that completes a color group costs **twice**
  the catalog price. The extra amount is paid to the bank. The rule applies to purchases
  from the bank, not to trades (4.8).
- **Auction**: all players (including the one who declined) may bid. Bids must be
  divisible by 20 and must exceed the current bid; the minimum first bid is 20 GE, and
  bids below the catalog price are allowed. The auction ends when no higher bid arrives
  within the countdown.
- **Auction completion rule**: if the winning tile completes a color group, the winner
  pays **120 % of its bid** instead of the bid.
- A tile nobody bids on stays with the bank and can be bought at the catalog price
  (including the double-price rule) by the next player who lands on it.
- The auction countdown is visible to all; every bid resets the countdown to the full
  15 seconds, measured from the bid time.

### 4.5 Rent

- Rent for a street depends on the number of houses/hotels built on it.
- **Doubled unbuilt rent**: if the owner holds the complete color group and the street is
  still unbuilt, the rent is **twice** the base rent (classic rule, "Miete bei ganzer
  Farbgruppe").
- A mortgaged property yields no rent.
- **Jail rule**: while a property owner sits in jail, that owner collects **no rent**.
  The tenant does not pay anything — no rent is due, no debt is created, nothing happens.
- Utilities: rent is 4× the dice total; if one player owns both utilities it is 10×.
- **Utility share (Subprime rule)** — part of the rent of the `rosa`, `orange`, `rot` and
  `gelb` streets flows to the utilities (no share for the other color groups, for Tankstellen
  or for the utilities themselves):
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
  - The share is computed on the rent actually collected — including a doubled unbuilt
    rent or a hotel rent — and paid out of that amount.

### 4.6 Building

- A player who owns a complete color group may build houses on its streets and hotels
  on them. The bank always has building material — there is no supply limit.
- Houses cost the catalog build cost. A **hotel counts as five houses**: building it
  costs the build cost once (as for a house) and the four houses standing on the
  property are returned to the bank.
- Selling a hotel back refunds `floor(buildCost / 2) × 1` GE — half the build price of
  the one house the hotel cost to build — and leaves the street with four houses.
- Building is nearly even within a color group: after every build the build levels of
  the group's streets (houses 0–4, a hotel counts as five) may differ by at most 1.
  Levels 2/2/1 are allowed, and so is a hotel on one street next to streets with four
  houses (5/4/4); 3/1/1 and a hotel next to streets with three houses (5/3/3) are not.
- Building happens in the pre-turn window (4.2), never while a move is pending.
- Houses/hotels can be sold back to the bank at half the build price each (a hotel for
  `floor(buildCost / 2)`, see above).

### 4.7 Property, mortgages, and selling

- **Mortgage**: the bank lends the owner **50 %** of the purchase price. A property can
  only be mortgaged when no street of its color group has houses or a hotel. A mortgaged
  property yields no rent and cannot be built on. To take the property back the owner
  pays **60 %** of the purchase price (50 % principal plus 10 % interest).
- **Selling to the bank**: the bank buys a property for **70 %** of its purchase price.
  Houses/hotels on it must be sold back first. A mortgaged property is returned to the
  bank for **nothing** — the 50 % mortgage is settled by the return.
- Both mechanisms are available (the German board has no mortgages, the international one
  has no selling to the bank — Subprime has both).
- Mortgaging and selling to the bank are also available while the active player decides on
  a purchase, settles a debt, or chooses a jail option, so they can raise the needed money
  before deciding (4.4, 4.10).
- A mortgaged or sold property returns to the bank and can be bought again from the bank
  at the catalog price (with the double-price rule if it completes a color group).

### 4.8 Trading

- Trading is allowed and happens **before rolling** (Subprime rule): the active player
  may trade in their pre-turn window; other players may trade with each other at any
  time.
- A player proposes a trade (tiles, cash and/or workshop passes in both directions);
  the other player accepts, declines, or counter-proposes — a counter-proposal carries
  the id of the offer it replaces, which the server then marks as countered so exactly
  one offer between two players is open at a time. Accepted trades are applied by the
  server atomically (ownership transfer plus cash transfer). All clients show every new
  offer as a dismissible popup with its full contents and whether they take part in it.
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
- **Debt**: a trade may only involve cash that both players actually have at the moment.
  The server rejects a trade whose cash leg either player cannot cover; a player who is
  already in debt cannot pay cash but may receive cash or trade properties. A player who
  cannot pay a rent, tax, card payment, or fine does not go bankrupt immediately — see the
  debt rules in 4.10.

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

### 4.10 Jail, debt, giving up, end of game

- **Jail**: three failed escape attempts, or a card, send a player to jail. On the third
  attempt the player must pay the jail fine (50 GE). Rolling a double while in jail
  leaves jail; otherwise the player stays and the turn passes. Leaving jail — by a
  double or after the mandatory fine — moves the player by the dice rolled in that same
  turn; a jail-exit double grants no extra turn. Before choosing a jail option the player
  may mortgage properties or sell buildings/properties to the bank to raise the fine.
- **Debt instead of automatic bankruptcy**: a payment that exceeds the player's cash is
  still booked in full. The payer's cash becomes negative and the creditor receives the
  full amount. The negative balance is a debt to the bank.
- While the active player's cash is negative the turn phase is `awaiting_debt`: the player
  cannot roll, buy, build, bid, unmortgage or conclude trades. They may mortgage
  properties, sell houses/hotels and sell properties to the bank until the balance is no
  longer negative. Every new income is offset against the debt. The turn ends
  automatically once cash is non-negative; a pending double does not grant another roll.
  Other players wait until the debt is settled. A player pushed into negative cash outside
  their own turn enters the debt phase when their turn starts.
- **Giving up**: a player may give up during their own turn — from the topbar in
  `awaiting_roll`, `awaiting_purchase` and `awaiting_jail`, or from the debt panel while
  their cash is negative. Both paths require a second confirmation. Giving up during
  `awaiting_purchase` discards the open offer; the tile stays with the bank and is not
  auctioned. The player leaves the game immediately: their buildings are sold back to the
  bank, remaining properties go to the bank, and the bank absorbs any remaining shortfall
  (the creditor was already paid). Get-out-of-jail cards return under their deck. Open
  trades are cancelled and the player is removed from any running auction. Giving up is
  not possible during an auction or outside the player's own turn.
- A player who gave up or was voted out leaves the game; their seat stays visible as
  eliminated and their turns are skipped.
- **End of game**: the game ends when only one player is left. The final screen shows the
  standings: net worth (cash + property value), rent collected/paid, and the winner.
- Disconnecting does not end the game: a disconnected player keeps their seat and the
  turn waits for them. Reconnecting with the same user id restores the seat.
- **Leaving a running game**: "Verlassen" only leaves the view. The seat stays in the
  game, `connected` becomes false and the turn waits for that player exactly as after a
  closed tab; rejoining with the room code reconnects the same seat and play continues.
  Leaving no longer forfeits.
- **Final leave from the start page**: "Raum verlassen" in the start-page banner (4.1)
  gives the seat up for good. In a running game it eliminates the player immediately,
  exactly like giving up and regardless of turn or debt: buildings are sold back to the
  bank, remaining properties go to the bank, get-out-of-jail cards return under their
  deck, open trades lapse and a running auction drops their bid; the turn advances when
  the player was active and the game ends when one player remains. A seat that is already
  eliminated is not eliminated again: its assets were already settled and its turn does
  not change, so the abandon only removes the stale membership. The membership is then
  removed, so the user can create, join and be invited again; the seat cannot be restored
  afterwards. In a lobby the seat is freed with the normal host promotion and
  last-member close, in a finished game the stale membership is removed and the room stays
  unchanged for everyone else. A caller without a seat is a no-op (204), an unknown or
  already closed room answers 404 `room_not_found`. The "Verlassen" on the room page still
  only pauses the seat (see above).
- **Votekick**: while the game is playing, every non-eliminated, currently connected
  player may vote to kick a paused (disconnected, non-eliminated) seat other than their
  own; the action is not offered in the lobby or in a finished game. The players list
  shows the tally per paused seat (`Votekick <votes>/<needed>`) and marks the viewer's
  own vote (`abgestimmt`); a repeated vote by the same player is idempotent. The kick
  fires immediately — no grace period and no host override — as soon as all eligible
  players have voted and at least one vote exists. The votes for a seat are cleared when
  that player reconnects or when the kick fires. Quorum is evaluated when a vote is
  cast: a voter who disconnects lowers the visible count but does not retroactively
  trigger a kick; the next vote re-evaluates against the current set. A successful kick
  eliminates the player exactly like giving up (buildings and properties back to the
  bank, seat visible as eliminated, turns skipped, game over when one player remains).

### 4.11 Spectators and the event log

- Users who open a running room without a seat watch as spectators and see the full
  board and the event log. **A spectator stays a spectator for the whole game** — they
  cannot take a seat later, not even after another player went bankrupt.
- A spectator pressing "Verlassen" returns them to the start page; in an open lobby
  the leave request is a no-op and a running game sends no request at all. It never
  changes a seat.
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
| Tankstelle / utility cards | next Tankstelle = double rent, next utility = 10× dice |
| Auction | declined tile goes to auction, bids in multiples of 20 |
| Color group completion | double price on purchase, 120 % of bid in auction |
| Rent in jail | owner collects no rent, tenant pays nothing |
| Unbuilt rent with a complete color group | doubled (classic rule) |
| Utility rent | 4× dice, 10× dice when both utilities are owned |
| Utility share | 10 % of rosa/orange street rent to Elektrizitätswerk, 10 % (down) of rot/gelb street rent to Wasserwerk |
| Mortgage | bank lends 50 %, unmortgage costs 60 %, only for a building-free group |
| Selling to the bank | 70 % of the purchase price; a mortgaged property returns for 0 |
| Buildings | levels within a group differ by at most 1; bank stock unlimited |
| Trading | allowed before rolling, unbuilt properties only, not judged |
| Debt | payments may push cash negative; the player must mortgage or sell before continuing |
| Autopilot | optional per seat and game; rolls, buys and builds automatically, pauses during auctions, stops on incoming trade offers and on decisions it cannot make |
| Giving up | own turn (not during an auction; the debt phase uses the debt panel), with a second confirmation; buildings/properties go to the bank, the bank absorbs the shortfall |
| Hotel | counts as five houses; costs the build cost once, four houses return to the bank; selling it back refunds half a build cost and leaves 4 houses |
| Card chaining | unlimited |
| Krumme Deals | forbidden by rule, not enforced in v1.0 |

### 4.13 Autopilot

- **Optional automation per seat**: the autopilot is a server-side helper that plays one
  seat and only executes the decisions it is configured for. It switches itself off as
  soon as a decision it cannot make is required, and it keeps playing while the tab is
  closed.
- **Settings**: stored per seat and per game, not persisted. `enabled` (default off),
  `buyAlways` (default on), `jail` (`none`/`pay`/`roll`, default `none`), `buildGroups`
  (default empty) and `liquidity` (default 0, 0–100000 GE). Only the owner sees the full
  settings; every other player and every spectator sees only whether the autopilot is
  active.
- **Pacing**: at most one action every 0.8 s, driven server-side by the room tick through
  `Game.Tick(now)`. A new turn continues after the same interval.
- **Rolling**: in `awaiting_roll` it rolls, including the follow-up roll after a double.
  Before rolling it builds when building is enabled and a legal candidate exists.
- **Buying**: with `buyAlways` it accepts every purchase it can pay for from current cash,
  including the double-price rule for a complete color group. If the cash is insufficient
  or `buyAlways` is off, it stops. It never declines an offer and therefore never starts
  an auction.
- **Jail (`Werkstatt`)**: `pay` uses a held get-out-of-jail card first and otherwise pays
  the fine if it is affordable; `roll` attempts a double; `none` stops. A selected option
  that cannot be executed stops it.
- **Building**: only in `awaiting_roll`, only in complete, unmortgaged color groups listed
  in `buildGroups`, and only while `cash − build cost ≥ liquidity` — the single shared
  liquidity field above the color switches. It picks the legal street with the fewest
  houses across all those groups (a hotel counts as 5); ties go to the lowest tile index.
  Even building within the group still applies.
- **Auction**: it never bids and never declines. It pauses while an auction runs and
  continues after the auction resolves as long as it is still enabled.
- **Trading**: an incoming offer — including a counter-offer — switches it off
  immediately; re-enabling it while the offer is open is allowed. Its own outgoing offer
  does not switch it off. It rolls at the start of its turn, so the pre-roll window for a
  trade is only about 0.8 s in practice; switching the autopilot off first is advisable,
  not required (the server does not enforce it).
- **Stops**: automatic stops are `autopilot_off` events with a `reason` (`trade_offer`,
  `purchase`, `jail`, `debt`), shown as a toast to the owner and in the event log for
  everyone. A manual switch-off uses `reason: "manual"` (event log only). Bankruptcy or
  elimination and game end switch it off silently.
- **Manual actions**: remain allowed and do not switch the autopilot off; it follows the
  new turn phase on the next tick — switching it off is the immediate takeover.
- **Illegal build groups**: a group that becomes incomplete or mortgaged is skipped, not a
  stop; building resumes once the group is legal again.

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
- Realtime client: native WebSocket with a small reconnecting wrapper; all game
  intents go over the same socket. A second reconnecting client with the same reconnect
  behaviour is used only by the start page and the room lobby for the presence channel;
  it is closed when the page switches scope.
- Entire UI in German; no i18n framework in v1.0.

### API shape

- REST for non-realtime operations: `POST /api/rooms` (create),
  `POST /api/rooms/{code}/join`, `POST /api/rooms/{code}/leave`,
  `POST /api/rooms/{code}/abandon`, `GET /api/rooms/{code}`, `GET /api/me`,
  `GET /api/me/room`, `GET /api/health`.
- `POST /api/rooms/{code}/leave` frees the caller's seat in an open lobby: no request
  body, 204 No Content. An unknown or closed room answers 404 `room_not_found`, a
  running game 403 `game_started`, a finished one 400 `game_over`; a user without a
  seat (spectator) is a no-op with 204.
- `GET /api/me/room` reports the caller's blocking room for the start-page banner (4.1):
  `{"room":null}` when no seat counts as "in a room", otherwise
  `{"room":{"code":"ABCDEF","phase":"lobby|playing|finished"}}`, using the same rule as
  create, join and invitations. A request without identity is 401, another method 405
  with `Allow: GET`.
- `POST /api/rooms/{code}/abandon` gives up the caller's own seat for good (4.10): no
  request body, 204 No Content with no response body. A lobby seat is freed like
  `leave` (host promotion, last-member close), a running game eliminates the player
  through the giving-up path and removes the membership, a finished game removes the
  stale membership. An unknown or closed room answers 404 `room_not_found` — the client
  treats that as success, like `leave` — another method 405 with `Allow: POST`; a caller
  without a seat is a no-op with 204.
- `POST /api/rooms` and `POST /api/rooms/{code}/join` reject a user who still holds a
  seat in a different live room with 403 `already_in_room` and a message naming that
  room's code; rejoining the same room stays allowed.
- WebSocket at `/api/ws?room=<code>` for everything that happens during play: the server
  sends the full game state after every change (small state, no diffing needed) plus
  discrete events for animations; the client sends intents (`roll`, `buy`, `decline`,
  `bid`, `build`, `sell_building`, `mortgage`, `unmortgage`, `sell_to_bank`,
  `propose_trade`, `respond_trade`, `jail_option`, `end_turn`, `set_autopilot`,
  `votekick`). `votekick` carries `targetId` (the paused seat to kick) and is answered
  with the usual `state` push or an `error`.
- Every room snapshot carries `seats` and `kickVotes` as arrays: `seats` is `[]` in an
  empty lobby (never `null`), and `kickVotes` is `[]` outside `playing`. While playing
  it holds one `{"userId","votes","voted"}` entry per paused, non-eliminated seat with
  at least one effective vote — the target, the count of valid votes and whether the
  requesting viewer already voted.
- `set_autopilot` carries the complete `autopilot` object (`enabled`, `buyAlways`,
  `jail`, `buildGroups`, `liquidity`); it is idempotent, allowed out of turn and in every
  turn phase for a seated player, and rejected for spectators. Every `PlayerView` carries
  `autopilot: {"active": bool, "config": {…}}`; `config` is only present in that seat's
  own snapshot — other players and spectators receive only `active`. Stops add a `reason`
  field to the `autopilot_off` event.
- Presence WebSocket at `/api/ws/presence[?room=<code>]`, the only place
  where presence and invitations travel. The display name is never part of the URL: the
  server starts with the identity name and only the `set_name` intent changes it.
  Without `room` it is the start-page connection
  (presence, invitation inbox, accept/decline/`set_name`); with `room` it is a
  room-lobby subscription that does not count as presence. Pushes:
  `{"type":"lobby","online":[…],"sent":[…]}` (room scope),
  `{"type":"inbox","invites":[…]}` (start page),
  `{"type":"accepted","room":{…}}` (reply to an accepted invitation) and
  `{"type":"error","error":{"code":"…","message":"…"}}`. Intents:
  `invite` (`userId`), `cancel_invite` / `accept_invite` / `decline_invite`
  (`inviteId`; accept also carries `name` and `color`) and `set_name` (`name`).
  Invitation validity is decided by the room: a background reconcile prunes stale
  invitations and accept re-checks the room.
- The room WebSocket contract is unchanged — `/api/ws` keeps `state`/`error` only.
  `set_name` renames only the caller and is rejected with `invalid_scope` on a
  room-scoped connection; an unknown `room` answers 404 before the upgrade.
- The message contract is defined once in Go structs and mirrored by TypeScript types in
  the frontend; no hand-written API schema file.
- **Response shape**: successful responses are plain domain objects (no envelope), e.g.
  `{"room":{"code":"ABCDEF", …}}`. **Errors** always use one shape:
  `{"error":{"message":"…","code":"…"}}` with an optional machine-readable `code`.
- **Status codes**: 401 missing or blank identity header · 403 not permitted
  (e.g. `game_started`, `already_in_room`) · 404 unknown room (also for rooms the caller
  may not see and for a closed lobby, so existence does not leak) · 405 known path,
  wrong method (with the `Allow` header) · 400 invalid request or state (e.g.
  `game_over`) · 204 No Content for a successful request without a body (leave, abandon)
  · 500 internal error (generic message; details only in the log) · 200 for health.

### Testing

- **Backend**: Go standard library only — no testify, no go-cmp. Table-driven tests with
  the loop variable named `tc` and the slice named `tests`; assertions use plain
  `t.Errorf("status = %d, want %d", …)`. HTTP tests use `net/http/httptest` and call the
  handler in-process (no network). Fakes are hand-written structs in the test file.
  Identity headers are set explicitly per request.
- **The rules engine is the test target**: the board catalog, rent ladders, the doubled
  unbuilt rent, the utility share and its rounding, the double-price and 120 %-of-bid
  rules, mortgage/sell arithmetic, auction increments, card effects, debt settlement and
  giving up are all covered by table-driven unit tests over the engine.
- **Presence and invitations** (`backend/internal/presence/`): the hub is tested
  in-process with a hand-written fake sink — deduplication of tabs, the online list,
  invite/cancel/decline/accept, `set_name`, scope errors, reconcile and accept-time
  validation. `backend/internal/api/` adds the end-to-end invite, accept, cancel and
  `set_name` flow over a real WebSocket.
- **Frontend**: Vitest with jsdom, `@testing-library/svelte` and `@testing-library/jest-dom`.
  Test files sit next to the code (`src/**/*.test.ts`), shared fixtures live in one
  testing module, and `fetch` is stubbed globally. The presence store
  (`src/lib/presence.test.ts`, fake WebSocket: scope switching, `set_name`, notices, the
  accept promise) and the lobby/invitation components (`LobbyInvites.test.ts`,
  `Invitations.test.ts`) are covered too.
- **Leave, abandon and votekick**: the always-array `seats` snapshot, the REST leave
  endpoint and its closed-room gates, the `GET /api/me/room` query (no room, lobby,
  running, finished without a connection, spectator, 401 and 405), the `abandon`
  endpoint (lobby host promotion and last-member close, elimination and turn advance in a
  running game, game end with one player left, stale seat in a finished game, non-member
  no-op, 404 and 405), the presence proof that an abandoned user becomes invitable
  again, votekick over the room WebSocket, and the `PlayersList` / `ActiveRoomBanner` /
  `session` Vitest tests cover the leave flow, the start-page banner and the vote
  surface.

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
- **Cards with their own rent rule**: kept as in the classic rules — the "next Tankstelle"
  card charges double rent, the "next utility" card charges 10× the dice total.
- **Get-out-of-jail card**: tradable like a property, offered in the trade engine
  alongside street cards.
- **Doubled unbuilt rent**: added after the board verification against the official
  German rulebook — if the owner holds the complete color group, an unbuilt street
  charges twice its base rent (classic rule, "Miete bei ganzer Farbgruppe"). It was
  missing from the first draft.
- **Tax field names**: corrected to the names on the official German board —
  *Einkommensteuer* and *Zusatzsteuer* (100 GE).
- **Einkommensteuer is a percentage tax**: field 4 takes 10 % of the player's cash
  instead of the classic flat 200 GE. Board data now carries either a fixed `tax`
  amount or a `taxPercent`, never both.
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
- **Frontend dev port**: 8000 - 8003 with identity mock.
- **Configuration**: environment variables only, no CLI flags — one source of truth, as in
  coach-leo. The variable table is in section 6.
- **Currency in the card texts**: the German cards print `M` (Monopoly Money); Subprime
  uses `GE`, so the printed amounts were converted. Card targets are stored as board
  positions, never as street names — the classic German board rotates the railway-station names
  and repeats a street name, so name-based lookup would resolve to the wrong tile.
- **Card catalog format** (3.2.2): every card carries `id`, `deck`, `text`, `effect`,
  `target`, `amount`, and `losSalary`; the loader validates 16 cards per deck and refuses
  to start on an invalid catalog.
- **Debt and giving up** (4.10): a payment a player cannot cover pushes their cash
  negative instead of ending the game. The creditor is paid in full; the player must
  mortgage or sell assets until the balance is non-negative, otherwise they give up with
  an explicit confirmation. On giving up, buildings are sold back at half the build price,
  properties and get-out-of-jail cards return to the bank/deck, and the bank absorbs the
  remaining shortfall. A player may also give up voluntarily in their own turn through the
  topbar button; it uses the same elimination path. Trades still may only use cash a player
  actually has.
- **Card chaining**: unlimited. In the German deck no card moves the player onto a card
  field, so chaining only starts via the "move back three tiles" card and always
  terminates.
- **Hotel**: counts as five houses — building it costs the build cost once and the four
  houses return to the bank (the sell-back refund, see below, is half of that one build
  price).
- **Bank building stock**: unlimited. The classic supply limit (32 houses, 12 hotels) was
  dropped by decision — building material is always available and "as long as the bank has
  the building material" is not a Subprime rule.
- **Even building**: relaxed from "all streets of the group at the same level" to "the build
  levels of a color group may differ by at most 1" (a hotel counts as five): 2/2/1 and 5/4/4
  are legal, 3/1/1 and 5/3/3 are not.
- **Hotel sell-back**: refunds `floor(buildCost / 2) × 1` and leaves the street with four
  houses. The earlier ×5 draft valued the refund at five houses and double-counted the four
  houses that were already paid for and returned to the bank when the hotel was built.
- **Jail exit**: leaving jail (by a double, or after the mandatory 50 GE fine on the third
  failed attempt) moves the player by the dice rolled in that turn; a jail-exit double grants
  no extra turn.
- **Utility share scope**: only the rent of the rosa/orange/rot/gelb streets flows to the
  Elektrizitätswerk / Wasserwerk — not the rent of the other color groups, Tankstellen or
  utilities.
- **Mortgage preconditions**: a property can only be mortgaged while its whole color group is
  free of buildings; a mortgaged property sold to the bank pays nothing (the mortgage is
  settled by the return).
- **Several creditors**: only reachable through the "Vorstand" card — every payee is credited
  in full immediately, even if this pushes the payer's cash negative. The payer then settles
  the resulting debt to the bank.
- **Unaffordable purchase**: a player who cannot cover the price still receives the offer and
  may raise money by mortgaging or selling property; the tile only goes to auction when the
  player actively starts the auction.
- **Dedicated presence channel**: presence and invitations travel over their own
  WebSocket (`/api/ws/presence`) instead of the room socket. The room contract stays
  `state`/`error`, and a global start-page list is not tied to a per-room connection.
  The room package exposes seat queries (`Room.InRoom`, `Manager.UserRoom`,
  `Manager.InRoomSeats`) that the presence hub calls, but there is no reverse
  dependency: nothing in the room package calls the hub. Cost: one more socket per tab
  and a second reconnect wrapper.
- **Invitation lifetime**: no TTL. An invitation ends through accept, decline,
  cancellation by the inviter, or the room starting, filling up or closing. An accept
  rejected for an invalid colour or because the invitee still sits in another room keeps
  the invitation, so it can be retried. Invitations live in memory and die with the
  process; an invitee who goes offline keeps their invitations.
- **Invitations only on the start page**: the persistent panel and the transient
  notice exist on the start page only. A user who is currently in a room cannot be
  invited to another room and is not part of the online list, so a second invitation
  surface is neither needed nor offered. A seat in a finished room does not count as
  "in a room", so a user who returns to the start page after a game is online and
  invitable again; a lobby seat still counts until the user leaves the room (the room
  page's `Verlassen`, which frees it over REST in a lobby, or the `leave_room` intent),
  even if the tab is closed.
- **Invitation rules**: at most one pending invitation per room and invitee; every room
  member may invite; only the sender may cancel. Accept and decline belong to the
  invitee alone.
- **Presence is query-validated**: the hub does not mirror room state. A 1 s reconcile
  prunes invitations whose room is gone, started, full or closed or that already
  contain the invitee, and accept re-checks the room at accept time (`game_started`,
  `room_full`, `room_not_found`). A stale accept therefore fails; only the display can
  lag by one tick.
- **Confirmed display name**: the start page sends `set_name` when the user confirms
  the name (`Namen übernehmen`/Enter) and re-sends it after every successful reconnect
  while it differs from the login name, so the online list and the invitee's name in the
  sent-invitations list follow a confirmed rename without a reload; the inviter name
  stored in an existing invitation (`fromName`) is fixed at creation. The intent renames
  only the caller and is rejected on a room-scoped connection. Unchanged lobby states
  are not re-pushed, so the reconcile tick does not flood clients.
- **Lobby leave goes through REST and returns to the start page**: "Verlassen" in an
  open lobby sends `POST /api/rooms/{code}/leave` (204), then closes the room connection
  and the room-presence subscription and navigates to `/`. The socket intent `leave_room`
  has no acknowledgement, so a request that fails silently would strand the user on a
  room page without presence; the REST call is independent of the socket and frees the
  seat even while the socket is down. Presence only exists on the start page, so the
  leaver must return there to be listed and invitable again. A failure other than an
  already-gone room keeps the seat and the user on the page rather than losing it
  silently; `leave_room` remains for compatibility and still frees a lobby seat.
- **One live room per user**: a seat in a room with an open room connection or an
  unfinished game (3.3) blocks creating or joining a different room with 403
  `already_in_room`, and the error names the caller's current room. Without the rule a
  user could hold two live seats; `Manager.UserRoom` and the presence query would then
  answer by map order, and a paused game would wait on a seat that plays elsewhere.
  Rejoining the same room and joining a finished game from the start page stay allowed.
- **An emptied lobby is closed**: when the last member leaves, the room is marked closed
  immediately and every lobby entry point (`GET /api/rooms/{code}`, join, the room
  WebSocket, the presence WebSocket and leave) answers 404 `room_not_found` until the
  abandoned-room cleanup removes the object. Map presence is not the gate, because the
  object survives for up to 30 s. This prevents a zombie room whose host has left and
  whose code can never start the game; the existing reconcile prunes its pending
  invitations.
- **Leaving a running game pauses the seat**: in `playing` and `finished`, "Verlassen"
  sends no intent and no REST request — it closes the socket and navigates, which is the
  same `connected=false` state as a closed tab. A rejoin with the code continues the same
  seat, so a player can step away without losing the game. Forfeiting happens through the
  two "Aufgeben" entry points — the topbar in the player's own turn and the debt panel
  while in debt — or through the legacy `leave_room` intent.
- **Unanimous votekick**: the other non-eliminated, currently connected players can kick
  a paused seat, and only when all of them voted (at least one). It is available
  immediately — no grace period, no host override — and a vote count below quorum is
  visible to everyone. Votes for a seat are dropped when it reconnects. Quorum is
  re-evaluated only when a vote is cast, so a voter disconnecting lowers the count
  without causing a surprise elimination. A successful kick uses the giving-up
  elimination path, so the seat stays visible as eliminated and the game ends when one
  player remains.
- **Final leave from the start page**: a seat that a closed tab or a paused running game
  leaves behind is shown on the start page and can be given up there (4.1), because
  otherwise the one-live-room rule would keep blocking create, join and invitations with
  no way out. The action uses its own `POST /api/rooms/{code}/abandon` endpoint instead of
  `/leave`, so `/leave` stays lobby-only and an existing client cannot forfeit a running
  game by accident. For a still-active seat, abandon reuses `Game.Leave`/`eliminate` — the
  single elimination path with the existing asset handling — and then removes the
  membership; an already-eliminated seat only loses its stale membership (no second
  elimination, no asset move, no turn change). Either way the user is free again; assets
  are not auctioned or redistributed and there is no end-game vote. The
  banner state is fetched over REST (`GET /api/me/room`) and refreshed on focus and
  visibility instead of being pushed over presence, which keeps the room package free of
  a new room-to-hub edge; a stale tab is corrected by the next refresh, and the server
  still rejects a create or join with `already_in_room` until the abandon succeeds. The
  one-live-room rule itself is unchanged.
- **Autopilot** (4.13): a server-side helper per seat inside the game state. It only calls
  the existing engine actions (`Roll`, `Buy`, `JailOption`, `Build`); its 0.8 s pacing
  comes from the 200 ms room tick. The full configuration travels through
  `set_autopilot`, other viewers see only `active`, and every stop is an `autopilot_off`
  event with a reason.

## 10. Remaining Assumptions (veto-able, no decision needed if accepted)

- **Spectators** may watch any room, also a finished one.
- **Connection loss** keeps the seat; the turn waits for the player, and in a running
  game the other players can vote the paused seat out (4.10).
- **Auction**: countdown 15 s, reset to 15 s on every bid; minimum first
  bid is 20 GE; bids in multiples of 20 and below the catalog price are allowed.
- **Bank stock** of building material: unlimited — houses and hotels are always available
  (the classic 32/12 supply limit was dropped).
- **Cards**: 16 + 16 cards, classic German decks, adapted so that all card payments go to
  the bank and the two player-to-player cards are exempt (3.2, 4.9).
- **Card register**: the German decks exist in an informal ("du") and a formal ("Sie")
  variant with identical effects. v1.0 uses the informal variant consistently.
- **Two card details are unverified against the sources** and are marked as such in the
  card catalog: the exact wording of the second "advance to the next Tankstelle" card, and
  whether the M50 dividend card is still part of the current German chance deck. Both
  have a documented fallback (see the card research notes).
- **Utility share rounding**: rounded down when both parties are players, rounded up
  when the share goes to the bank.
- **Jail fine**: 50 GE; three failed escape attempts, then the fine is mandatory.
- **Room codes**: 6 uppercase letters, unambiguous alphabet (no O/0, I/1).
- **Language**: UI and all game content in German; repository, spec, code, and comments
  in English.
- **Ports**: backend 8080, frontend 8081, reverse proxy with location mock starting from 8000.
- **Deployment**: Docker compose behind Timo's reverse proxy, oauth2-proxy as the only
  ingress; production compose stays outside the repository, only `docker-compose-dev.yaml`
  is committed. The dev stack mocks the proxy with a Caddy service that injects the
  identity headers, as in coach-leo.
- **Container images**: the runtime image is `madebytimo/base`, with `madebytimo/nodejs`
  for the frontend build and `madebytimo/go` for the backend build. A Go healthcheck
  binary is compiled in, since the runtime image has no shell.
