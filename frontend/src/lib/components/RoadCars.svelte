<script lang="ts">
	import type { GameEvent, Player } from '#lib/types';
	import { tokenColor } from '#lib/format';
	import {
		PARKING_BAYS,
		PARKING_BAY_DEPTH,
		PARKING_BAY_WIDTH,
		PARKING_CORNER,
		ROAD_WIDTH,
		driveDirection,
		junction,
		parkingBay,
		parkingEntry,
		parksOnCorner,
		spurEnd,
		type MoveKind
	} from '#lib/road';
	import CarIcon from './CarIcon.svelte';
	import { onMount } from 'svelte';

	let {
		players,
		currentSeat,
		events,
		selectedSeat = null
	}: {
		players: Player[];
		currentSeat: number;
		events: GameEvent[];
		selectedSeat?: number | null;
	} = $props();

	// Geometry of the centre ring road, as fractions of the middle area. These
	// values mirror road.ts: the road is ROAD wide and the corner spurs reach
	// into the corner fields.
	const ROAD = ROAD_WIDTH / 100;

	let layer: HTMLDivElement | undefined = $state();
	let ready = $state(false);
	let centers = $state<Array<{ x: number; y: number }>>([]);
	// Outermost corner of every corner field, where a car in the workshop parks.
	let corners = $state<Array<{ x: number; y: number }>>([]);
	let carWidth = $state(0.031);

	interface Point {
		x: number;
		y: number;
		angle: number;
		// Unit vector along the road (direction of travel).
		dx: number;
		dy: number;
		// Unit vector across the road (towards the middle of the board).
		px: number;
		py: number;
	}

	const SPUR_DIRS: Record<number, [number, number]> = {
		0: [1, 1],
		10: [-1, 1],
		20: [-1, -1],
		30: [1, -1]
	};

	// Cumulative heading in the middle of the corner arc, i.e. half a turn
	// between the two straights. The spin offset keeps the car turning the same
	// way around the whole board.
	const CORNER_ANGLE: Record<number, number> = { 0: 225, 10: 315, 20: 405, 30: 495 };
	// Extra cars on a corner queue up along the following straight.
	const CORNER_EXIT: Record<number, [number, number]> = {
		0: [-1, 0],
		10: [0, -1],
		20: [1, 0],
		30: [0, 1]
	};

	/** Position of a field on the ring road. */
	function ringPoint(index: number): Point {
		const half = ROAD / 2;
		const center = centers[index];
		if (!center) {
			return { x: 0.5, y: 0.5, angle: 270, dx: -1, dy: 0, px: 0, py: 1 };
		}
		// Corner fields sit in the middle of the junction arc, right where the
		// spur leaves the ring.
		if (index % 10 === 0) {
			const point = junction(index);
			const [dx, dy] = CORNER_EXIT[index] ?? [-1, 0];
			return { x: point.x / 100, y: point.y / 100, angle: CORNER_ANGLE[index], dx, dy, px: -dy, py: dx };
		}
		if (index < 10) return { x: center.x, y: 1 - half, angle: 270, dx: -1, dy: 0, px: 0, py: 1 };
		if (index < 20) return { x: half, y: center.y, angle: 360, dx: 0, dy: -1, px: 1, py: 0 };
		if (index < 30) return { x: center.x, y: half, angle: 450, dx: 1, dy: 0, px: 0, py: 1 };
		return { x: 1 - half, y: center.y, angle: 540, dx: 0, dy: 1, px: 1, py: 0 };
	}

	/** Parking spot inside a corner field. A workshop visitor stops at the end
	    of the spur like any other corner; a player actually in the workshop
	    drives on to the outermost corner of the field. */
	function spurPoint(index: number, inWorkshop: boolean): Point {
		const [sx, sy] = SPUR_DIRS[index] ?? [1, 1];
		const base = ringPoint(index);
		let end = { x: spurEnd(index).x / 100, y: spurEnd(index).y / 100 };
		if (index === 10 && inWorkshop) {
			const corner = corners[10] ?? end;
			// Keep the whole car inside the field instead of half of it hanging
			// off the board edge.
			const inset = carWidth * 0.6;
			end = { x: corner.x - sx * inset, y: corner.y - sy * inset };
		}
		return {
			x: end.x,
			y: end.y,
			angle: base.angle - 90,
			dx: sx / Math.SQRT2,
			dy: sy / Math.SQRT2,
			px: -sy / Math.SQRT2,
			py: sx / Math.SQRT2
		};
	}

	// Measure the real fields so every car sits exactly on the centre line of
	// its field, regardless of grid gaps and track sizes. Positions are stored
	// as fractions of the middle area, which keeps them stable under page zoom.
	onMount(() => {
		const el = layer;
		const centre = el?.parentElement;
		const board = centre?.parentElement;
		if (!el || !centre || !board) return;

		const measure = () => {
			const box = centre.getBoundingClientRect();
			const local = centre.offsetWidth || box.width;
			const zoom = local > 0 ? box.width / local : 1;
			const next: Array<{ x: number; y: number }> = [];
			const cornerPoints: Array<{ x: number; y: number }> = [];
			for (const tile of board.querySelectorAll<HTMLElement>('.tile[data-index]')) {
				const rect = tile.getBoundingClientRect();
				const index = Number(tile.dataset.index);
				const left = (rect.left - box.left) / zoom / local;
				const top = (rect.top - box.top) / zoom / local;
				const right = (rect.right - box.left) / zoom / local;
				const bottom = (rect.bottom - box.top) / zoom / local;
				next[index] = { x: (left + right) / 2, y: (top + bottom) / 2 };
				if (index % 10 === 0) {
					const [sx, sy] = SPUR_DIRS[index] ?? [1, 1];
					cornerPoints[index] = { x: sx > 0 ? right : left, y: sy > 0 ? bottom : top };
				}
			}
			centers = next;
			corners = cornerPoints;
			// Car width follows the clamp() in the style block below.
			const boardWidth = board.offsetWidth || local;
			carWidth = Math.min(Math.max(0.023 * boardWidth, 19.2), 28.8) / local;
			ready = true;
		};

		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(centre);
		window.addEventListener('resize', measure);
		return () => {
			observer.disconnect();
			window.removeEventListener('resize', measure);
		};
	});

	// The displayed position lags behind the authoritative position so the car
	// can drive tile by tile along the road instead of jumping. It starts empty:
	// until the first client tick the cars simply sit on their real fields.
	let display = $state<number[]>([]);
	// Extra full turns, so wrapping from field 39 to field 0 keeps rotating in
	// the same direction instead of spinning backwards.
	let spin = $state<number[]>([]);
	// Marked bay each seat occupies on the Frei-Parken lot. A bay holds one car
	// at the front and a second one nose-to-tail at the back, so up to
	// 2 × PARKING_BAYS cars fit. The slot is kept until the car drives off.
	let baySlot = $state<number[]>([]);
	// Marks the tick on which a car leaves its bay, so the drive out of the lot
	// keeps the softer parking transition instead of snapping onto the ring.
	let bayExit = $state<boolean[]>([]);
	// 0 while a car is on the driveway apron, 1 once it sits in its bay. The
	// one-tick stop at the lot entrance keeps the approach on the asphalt.
	let bayAge = $state<number[]>([]);

	/** First free bay slot. A car parks deep first, so a second car can pull in
	    in front of it without crossing it. */
	function nextBaySlot(taken: Set<number>): number {
		for (let bay = 0; bay < PARKING_BAYS; bay++) {
			if (!taken.has(bay) && !taken.has(PARKING_BAYS + bay)) return PARKING_BAYS + bay;
		}
		for (let bay = 0; bay < PARKING_BAYS; bay++) {
			if (!taken.has(bay)) return bay;
			if (!taken.has(PARKING_BAYS + bay)) return PARKING_BAYS + bay;
		}
		return -1;
	}

	/** Where the car of a bay slot sits. Cars park nose-in towards the back;
	    when a bay is shared, the two cars move apart and shrink. */
	function bayPoint(slot: number, shared: boolean): Point {
		const center = parkingBay(slot % PARKING_BAYS);
		const deep = slot >= PARKING_BAYS;
		const shift = shared ? (deep ? -PARKING_BAY_DEPTH * 0.24 : PARKING_BAY_DEPTH * 0.15) : 0;
		return {
			x: center.x / 100,
			y: (center.y + shift) / 100,
			angle: 0,
			dx: 0,
			dy: -1,
			px: 1,
			py: 0
		};
	}

	// The newest move the server logged for a seat. The event log is the only
	// place that knows whether a move was a card going backwards or a trip to
	// jail, and both need a different route than a normal forward move.
	function lastMoveKind(seat: number): MoveKind {
		for (let i = events.length - 1; i >= 0; i--) {
			const event = events[i];
			if (event.player !== seat) continue;
			if (event.type === 'move_back') return 'backward';
			if (event.type === 'move') return 'forward';
			if (event.type === 'jail') return 'jail';
		}
		return 'forward';
	}

	onMount(() => {
		for (const player of players) {
			display[player.seat] = player.position;
			spin[player.seat] = 0;
		}
		display = [...display];
		spin = [...spin];

		// A car that is already parked on Frei-Parken takes its bay right away.
		const taken = new Set<number>();
		for (const player of players) {
			if (player.bankrupt || player.position !== PARKING_CORNER) continue;
			const slot = nextBaySlot(taken);
			if (slot >= 0) {
				baySlot[player.seat] = slot;
				bayAge[player.seat] = 1;
				taken.add(slot);
			}
		}
		baySlot = [...baySlot];
		bayAge = [...bayAge];

		const step = (seat: number, next: number) => {
			const previous = display[seat];
			if (previous === 39 && next === 0) spin[seat] = (spin[seat] ?? 0) + 360;
			if (previous === 0 && next === 39) spin[seat] = (spin[seat] ?? 0) - 360;
			display[seat] = next;
		};

		const tick = () => {
			let changed = false;
			for (const player of players) {
				if (player.bankrupt) continue;
				const seat = player.seat;
				const current = display[seat];
				if (current === undefined) {
					display[seat] = player.position;
					changed = true;
					continue;
				}
				if (current === player.position) continue;

				// Never snap: every move is driven along the ring, however far
				// the player was moved.
				const direction = driveDirection(current, player.position, lastMoveKind(seat));
				step(seat, (current + direction + 40) % 40);
				changed = true;
			}

			// Keep one marked bay per car that is standing on the Frei-Parken
			// field. A car that is still driving past never gets a bay, and a
			// car that just left keeps its bay for the drive-out tick.
			const onField = (player: Player) =>
				!player.bankrupt && (display[player.seat] ?? player.position) === PARKING_CORNER;
			const parkedNow = (player: Player) =>
				onField(player) &&
				(player.position === PARKING_CORNER || (baySlot[player.seat] ?? -1) >= 0);
			// Let a freshly assigned car spend one tick on the driveway apron
			// before it slides into its bay.
			let agesChanged = false;
			for (const player of players) {
				if ((baySlot[player.seat] ?? -1) < 0) continue;
				if ((bayAge[player.seat] ?? 1) < 1) {
					bayAge[player.seat] = (bayAge[player.seat] ?? 0) + 1;
					agesChanged = true;
				}
			}
			const taken = new Set<number>();
			for (const player of players) {
				const slot = baySlot[player.seat] ?? -1;
				if (slot >= 0 && parkedNow(player)) taken.add(slot);
			}
			let slotsChanged = false;
			for (const player of players) {
				const slot = baySlot[player.seat] ?? -1;
				const parked = parkedNow(player);
				if (slot >= 0 && !parked) {
					baySlot[player.seat] = -1;
					bayExit[player.seat] = true;
					slotsChanged = true;
				} else if (slot < 0 && bayExit[player.seat]) {
					// The car is already on the ring; drop the soft transition
					// again now that the drive out has been rendered.
					bayExit[player.seat] = false;
					slotsChanged = true;
				}
				if (parked && slot < 0) {
					const next = nextBaySlot(taken);
					if (next >= 0) {
						baySlot[player.seat] = next;
						bayAge[player.seat] = 0;
						bayExit[player.seat] = false;
						taken.add(next);
						slotsChanged = true;
					}
				}
			}
			if (slotsChanged) {
				baySlot = [...baySlot];
				bayExit = [...bayExit];
				bayAge = [...bayAge];
			} else if (agesChanged) {
				bayAge = [...bayAge];
			}

			if (changed) {
				display = [...display];
				spin = [...spin];
			}
		};

		const timer = setInterval(tick, 75);
		return () => clearInterval(timer);
	});

	interface CarView {
		id: string;
		x: number;
		y: number;
		angle: number;
		scale: number;
		initial: string;
		color: string;
		active: boolean;
		parked: boolean;
		bay: boolean;
		name: string;
		faded: boolean;
	}

	const cars = $derived.by(() => {
		if (!ready) return [];

		// Cars on the same field share a group. A car that ends its move on a
		// corner turns into the field; a car passing through stays on the ring,
		// so the two states never look alike. The workshop is special: a visitor
		// stops at the end of the spur like every other corner, while a player
		// actually in the workshop drives on to the outermost corner.
		const groups = new Map<string, { point: Point; list: Player[]; parked: boolean }>();
		const result: CarView[] = [];
		// Parked cars shrink to the marked bay, keeping the painted lines visible.
		const bayFit = Math.min(
			0.9,
			((PARKING_BAY_WIDTH / 100) * 0.82) / carWidth,
			((PARKING_BAY_DEPTH / 100) * 0.82) / (carWidth * (52 / 30))
		);
		// How many cars share each bay, so a shared bay can shrink both.
		const bayUse = new Map<number, number>();
		for (const player of players) {
			if (player.bankrupt) continue;
			const slot = baySlot[player.seat] ?? -1;
			if (slot < 0) continue;
			const bay = slot % PARKING_BAYS;
			bayUse.set(bay, (bayUse.get(bay) ?? 0) + 1);
		}
		for (const player of players) {
			if (player.bankrupt) continue;
			const position = display[player.seat] ?? player.position;
			const parked = parksOnCorner(position, player.position);
			const slot = baySlot[player.seat] ?? -1;
			// A car that ends its move on Frei-Parken pulls into its own bay.
			if (parked && position === PARKING_CORNER && slot >= 0) {
				const deep = slot >= PARKING_BAYS;
				const shared = (bayUse.get(slot % PARKING_BAYS) ?? 0) > 1;
				const entering = (bayAge[player.seat] ?? 1) < 1;
				const point = entering
					? {
							x: parkingEntry().x / 100,
							y: parkingEntry().y / 100,
							angle: 0,
							dx: 0,
							dy: -1,
							px: 1,
							py: 0
						}
					: bayPoint(slot, shared);
				const scale = bayFit * (shared ? (deep ? 0.48 : 0.62) : 1);
				result.push({
					id: player.id,
					x: point.x,
					y: point.y,
					angle: point.angle + (spin[player.seat] ?? 0),
					scale,
					initial: scale >= 0.8 ? player.name.slice(0, 1).toUpperCase() : '',
					color: tokenColor(player.color),
					active: player.seat === currentSeat,
					parked: true,
					bay: true,
					name: player.name,
					faded: selectedSeat !== null && player.seat !== selectedSeat
				});
				continue;
			}
			const inWorkshop = parked && position === 10 && player.inJail;
			const key = `${position}:${parked ? (inWorkshop ? 'corner' : 'spur') : 'ring'}`;
			let group = groups.get(key);
			if (!group) {
				group = {
					point: parked ? spurPoint(position, inWorkshop) : ringPoint(position),
					list: [],
					parked
				};
				groups.set(key, group);
			}
			group.list.push(player);
		}

		for (const group of groups.values()) {
			group.list.sort((a, b) => a.seat - b.seat);
			const count = group.list.length;
			const scale = count > 6 ? 0.66 : count > 3 ? 0.82 : 1;
			const columns = count > 1 ? 2 : 1;
			const rows = Math.ceil(count / columns);
			const width = carWidth * scale;
			const across = Math.min(width * 1.08, ROAD * 0.94) / 2;
			const along = width * 1.85;

			group.list.forEach((player, i) => {
				const column = i % columns;
				const row = Math.floor(i / columns);
				const shiftAlong = (row - (rows - 1) / 2) * along;
				const shiftAcross = (column - (columns - 1) / 2) * across * 2;
				result.push({
					id: player.id,
					x: group.point.x + group.point.dx * shiftAlong + group.point.px * shiftAcross,
					y: group.point.y + group.point.dy * shiftAlong + group.point.py * shiftAcross,
					angle: group.point.angle + (spin[player.seat] ?? 0),
					scale,
					initial: scale >= 0.8 ? player.name.slice(0, 1).toUpperCase() : '',
					color: tokenColor(player.color),
					active: player.seat === currentSeat,
					parked: group.parked,
					bay: bayExit[player.seat] ?? false,
					name: player.name,
					faded: selectedSeat !== null && player.seat !== selectedSeat
				});
			});
		}
		return result;
	});
</script>

<div class="cars-layer" bind:this={layer} aria-label="Spielerautos">
	{#each cars as car (car.id)}
		<span
			class="car"
			class:active={car.active}
			class:parked={car.parked}
			class:bay={car.bay}
			class:faded={car.faded}
			title={car.name}
			style="left: {car.x * 100}%; top: {car.y * 100}%; transform: translate(-50%, -50%) rotate({car.angle}deg) scale({car.scale});"
		>
			<CarIcon color={car.color} initial={car.initial} active={car.active} />
		</span>
	{/each}
</div>

<style>
	.cars-layer {
		position: absolute;
		inset: 0;
		z-index: 1;
		pointer-events: none;
	}

	.car {
		position: absolute;
		width: clamp(1.2rem, 2.3cqw, 1.8rem);
		line-height: 0;
		transition:
			left 75ms linear,
			top 75ms linear,
			transform 75ms linear;
		filter:
			drop-shadow(0 0 0.12rem color-mix(in srgb, var(--cd-ral9016) 45%, transparent))
			drop-shadow(0 2px 2px color-mix(in srgb, var(--md-sys-color-shadow) 50%, transparent));
		will-change: left, top, transform;
	}

	/* Only the focused player's car stays at full strength. */
	.car.faded {
		opacity: 0.45;
	}

	/* Turning into a corner field is slower and softer than driving on. */
	.car.parked {
		z-index: 2;
		transition:
			left 220ms ease-in-out,
			top 220ms ease-in-out,
			transform 220ms ease-in-out;
	}

	/* Pulling into or out of a marked bay gets the same soft motion, even on
	   the tick where the car is already back on the ring. */
	.car.bay {
		transition:
			left 220ms ease-in-out,
			top 220ms ease-in-out,
			transform 220ms ease-in-out;
	}
</style>
