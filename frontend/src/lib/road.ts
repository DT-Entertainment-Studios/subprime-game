/**
 * Geometry of the ring road in the middle of the board.
 *
 * All values are percentages of the middle area (0…100), so the road scales
 * with the board. Board.svelte renders these paths as SVG and RoadCars.svelte
 * uses the same numbers to place the cars, which keeps cars and asphalt in
 * sync.
 *
 * The ring is a rounded rectangle. At every corner a spur leaves the ring at
 * 45° towards the corner field. Instead of stacking two straight roads, the
 * ring edge opens up at the junction and the spur edge flows into it with a
 * fillet, so each corner reads as one intersection.
 */

/** Width of the asphalt. */
export const ROAD_WIDTH = 7;
/**
 * Radius of the ring centre line at the corners.
 *
 * Kept tight on purpose: the outer bend eats into the lane, and a car parked on
 * a field next to a corner field would hang off the asphalt at a larger radius.
 */
export const CORNER_RADIUS = 4;
/** Width of a spur. Narrower than the ring, so the junction stays readable. */
export const SPUR_WIDTH = 5;
/** How far a spur reaches into the corner field, per axis. */
export const SPUR_REACH = 4;
/** Width of the white edge lines. */
export const EDGE_WIDTH = 0.35;
/** Width of the centre markings. */
export const MARK_WIDTH = 0.3;

/** How a move reached its field, as logged by the server. */
export type MoveKind = 'forward' | 'backward' | 'jail';

/**
 * Direction a car drives while it animates a move along the ring.
 *
 * Cards that send a player back are driven backwards, jail takes the shorter
 * way around the ring, and every other move is a normal forward drive, however
 * many fields it covers. Cars therefore never jump.
 */
export function driveDirection(from: number, to: number, kind: MoveKind): 1 | -1 {
	if (kind === 'backward') return -1;
	if (kind !== 'jail') return 1;

	const forward = (to - from + 40) % 40;
	const backward = (from - to + 40) % 40;
	return backward <= forward ? -1 : 1;
}

/** Whether a car that has finished its move turns into a corner field. */
export function parksOnCorner(display: number, position: number): boolean {
	return display === position && position % 10 === 0;
}

const HALF = ROAD_WIDTH / 2;
const OUTER = CORNER_RADIUS + HALF;
const INNER = CORNER_RADIUS - HALF;
const SPUR_HALF = SPUR_WIDTH / 2;
/** Half the opening angle of a junction mouth on the outer edge. */
const MOUTH = Math.asin(SPUR_HALF / OUTER);
/** Radius of the fillets that blend a spur into the ring edge. */
const FILLET = 1.2;
/** Gap between a junction and the next centre marking. */
const MARK_GAP = 0.11;

const D = Math.SQRT1_2;

interface Corner {
	/** Centre of the rounded corner. */
	cx: number;
	cy: number;
	/** Start angle of the corner arc. */
	a0: number;
	/** Direction of the spur, pointing into the corner field. */
	dx: number;
	dy: number;
}

export const CORNERS: Record<number, Corner> = {
	0: { cx: 100 - HALF - CORNER_RADIUS, cy: 100 - HALF - CORNER_RADIUS, a0: 0, dx: D, dy: D },
	10: { cx: HALF + CORNER_RADIUS, cy: 100 - HALF - CORNER_RADIUS, a0: Math.PI / 2, dx: -D, dy: D },
	20: { cx: HALF + CORNER_RADIUS, cy: HALF + CORNER_RADIUS, a0: Math.PI, dx: -D, dy: -D },
	30: {
		cx: 100 - HALF - CORNER_RADIUS,
		cy: HALF + CORNER_RADIUS,
		a0: (3 * Math.PI) / 2,
		dx: D,
		dy: -D
	}
};

export const CORNER_ORDER = [0, 10, 20, 30] as const;

const round = (n: number) => Math.round(n * 100) / 100;

function at(corner: Corner, radius: number, angle: number) {
	return {
		x: round(corner.cx + radius * Math.cos(angle)),
		y: round(corner.cy + radius * Math.sin(angle))
	};
}

/** Point where a spur leaves the ring centre line. */
export function junction(index: number) {
	const corner = CORNERS[index];
	return at(corner, CORNER_RADIUS, corner.a0 + Math.PI / 4);
}

/** Centre of the spur end, inside the corner field. */
export function spurEnd(index: number) {
	const corner = CORNERS[index];
	return {
		x: round((corner.dx > 0 ? 100 : 0) + Math.sign(corner.dx) * SPUR_REACH),
		y: round((corner.dy > 0 ? 100 : 0) + Math.sign(corner.dy) * SPUR_REACH)
	};
}

/**
 * Fillet between the ring edge and one side of a spur.
 *
 * The fillet is a circle of radius {@link FILLET} that touches the outer edge
 * of the ring and the spur side at the same point of tangency, so the kerb runs
 * into the junction without a kink. `side` is +1 for the side the spur's first
 * cap end sits on, -1 for the other.
 */
function fillet(key: number, side: 1 | -1) {
	const corner = CORNERS[key];
	// Unit vector across the spur, away from the spur's centre line.
	const nx = side * corner.dy;
	const ny = -side * corner.dx;
	// Offset of the fillet centre from the corner centre, along the spur and
	// across it: it sits outside the ring on the far side of the spur edge.
	const along = Math.sqrt((OUTER + FILLET) ** 2 - (SPUR_HALF + FILLET) ** 2);
	const cx = corner.cx + along * corner.dx + (SPUR_HALF + FILLET) * nx;
	const cy = corner.cy + along * corner.dy + (SPUR_HALF + FILLET) * ny;

	return {
		// Where the ring edge line hands over to the fillet.
		ring: {
			x: round(corner.cx + ((cx - corner.cx) * OUTER) / (OUTER + FILLET)),
			y: round(corner.cy + ((cy - corner.cy) * OUTER) / (OUTER + FILLET))
		},
		// Where the fillet hands over to the straight spur side.
		side: { x: round(cx - FILLET * nx), y: round(cy - FILLET * ny) }
	};
}

/**
 * Geometry of one junction mouth: where the outer edge line opens up, how the
 * fillets bend into the spur sides and where the spur ends.
 */
function spurMouth(key: number) {
	const corner = CORNERS[key];
	// Unit vector across the spur, towards the corner of the board.
	const nx = corner.dy;
	const ny = -corner.dx;
	const end = spurEnd(key);
	const inSide = fillet(key, 1);
	const outSide = fillet(key, -1);

	return {
		// Where the outer edge line stops on each side of the mouth.
		edgeIn: inSide.ring,
		edgeOut: outSide.ring,
		// Ends of the fillets, where the straight spur sides begin.
		a2: inSide.side,
		b2: outSide.side,
		capA: { x: round(end.x + SPUR_HALF * nx), y: round(end.y + SPUR_HALF * ny) },
		capB: { x: round(end.x - SPUR_HALF * nx), y: round(end.y - SPUR_HALF * ny) }
	};
}

/**
 * Mouth of a junction: fillet, spur side, rounded cap, other side and fillet.
 *
 * Starts and ends on the outer edge line, so it slots in between the two arcs
 * of the ring edge that remain on either side of the mouth.
 */
function mouthPath(key: number): string {
	const m = spurMouth(key);
	return [
		`A ${FILLET} ${FILLET} 0 0 0 ${m.a2.x} ${m.a2.y}`,
		`L ${m.capA.x} ${m.capA.y}`,
		`A ${SPUR_HALF} ${SPUR_HALF} 0 0 1 ${m.capB.x} ${m.capB.y}`,
		`L ${m.b2.x} ${m.b2.y}`,
		`A ${FILLET} ${FILLET} 0 0 0 ${m.edgeOut.x} ${m.edgeOut.y}`
	].join(' ');
}

/** Outline of the whole road surface, including the four spur mouths. */
function areaPath(): string {
	let d = '';

	CORNER_ORDER.forEach((key, i) => {
		const corner = CORNERS[key];
		const mouth = spurMouth(key);
		const start = at(corner, OUTER, corner.a0);
		const cornerEnd = at(corner, OUTER, corner.a0 + Math.PI / 2);

		d += i === 0 ? `M ${start.x} ${start.y} ` : `L ${start.x} ${start.y} `;
		d += `A ${OUTER} ${OUTER} 0 0 1 ${mouth.edgeIn.x} ${mouth.edgeIn.y} `;
		d += `${mouthPath(key)} `;
		d += `A ${OUTER} ${OUTER} 0 0 1 ${cornerEnd.x} ${cornerEnd.y} `;
	});

	return `${d.trim()} Z`;
}

/** Inner edge of the ring, a rounded rectangle with a hole. */
function innerPath(): string {
	const lo = ROAD_WIDTH;
	const hi = 100 - ROAD_WIDTH;
	const r = INNER;
	return [
		`M ${lo + r} ${lo}`,
		`H ${hi - r}`,
		`A ${r} ${r} 0 0 1 ${hi} ${lo + r}`,
		`V ${hi - r}`,
		`A ${r} ${r} 0 0 1 ${hi - r} ${hi}`,
		`H ${lo + r}`,
		`A ${r} ${r} 0 0 1 ${lo} ${hi - r}`,
		`V ${lo + r}`,
		`A ${r} ${r} 0 0 1 ${lo + r} ${lo}`,
		'Z'
	].join(' ');
}

/** Dashed centre line: one run per side, interrupted at every junction. */
function ringMarks(): string {
	let d = '';

	CORNER_ORDER.forEach((key, i) => {
		const corner = CORNERS[key];
		const next = CORNERS[CORNER_ORDER[(i + 1) % 4]];

		const from = at(corner, CORNER_RADIUS, corner.a0 + Math.PI / 4 + MOUTH + MARK_GAP);
		const to = at(corner, CORNER_RADIUS, corner.a0 + Math.PI / 2);
		const nextStart = at(next, CORNER_RADIUS, next.a0);
		const nextEnd = at(next, CORNER_RADIUS, next.a0 + Math.PI / 4 - MOUTH - MARK_GAP);

		d += `M ${from.x} ${from.y} `;
		d += `A ${CORNER_RADIUS} ${CORNER_RADIUS} 0 0 1 ${to.x} ${to.y} `;
		d += `L ${nextStart.x} ${nextStart.y} `;
		d += `A ${CORNER_RADIUS} ${CORNER_RADIUS} 0 0 1 ${nextEnd.x} ${nextEnd.y} `;
	});

	return d.trim();
}

/** Short centre markings on the spurs, clear of the junction mouth. The
    Frei-Parken spur ends in the parking lot, so it gets no dashed centre line. */
function spurMarks(): string {
	let d = '';

	for (const key of CORNER_ORDER) {
		if (key === PARKING_CORNER) continue;
		const corner = CORNERS[key];
		const from = at(corner, OUTER + 3, corner.a0 + Math.PI / 4);
		const end = spurEnd(key);
		const to = {
			x: round(end.x - 2.2 * corner.dx),
			y: round(end.y - 2.2 * corner.dy)
		};
		d += `M ${from.x} ${from.y} L ${to.x} ${to.y} `;
	}

	return d.trim();
}

/* --- Parking lot at the Frei-Parken field ------------------------------
 *
 * Only the Frei-Parken corner has a real parking lot: a driveway along the
 * inner edge of the corner field with a row of marked bays on its far side.
 * The lot stays clear of the field border, so no asphalt spills into the
 * neighbouring Bankstraße and Ulmenplatz fields. All coordinates stay in the
 * middle-area space; the field lies in the negative quadrant next to the
 * board corner.
 */

/** Board index of the corner field that carries the parking lot. */
export const PARKING_CORNER = 20;
/** Marked bays in the row. */
export const PARKING_BAYS = 5;
/** Bay pitch along the row. */
export const PARKING_BAY_WIDTH = 2.9;
/** Left end of the bay row. */
export const PARKING_ROW_LEFT = -15.7;
/** Right end of the bay row, clear of the lot's right edge. */
export const PARKING_ROW_RIGHT = PARKING_ROW_LEFT + PARKING_BAYS * PARKING_BAY_WIDTH;
/** Bay depth from the driveway kerb to the back line. */
export const PARKING_BAY_DEPTH = 6.6;
/** Edge where the bays meet the driveway. */
export const PARKING_KERB = -5;
/** Back line of the bays (deep inside the field). */
export const PARKING_BACK = PARKING_KERB - PARKING_BAY_DEPTH;
/**
 * How far the asphalt keeps clear of the field's inner edges.
 *
 * The grid gap plus the field border take roughly 0.3–0.7 units of middle-area
 * space depending on the board size. Staying outside that range keeps the
 * field border visible and stops the lot from reaching the neighbouring
 * Bankstraße and Ulmenplatz fields.
 */
export const PARKING_INSET = 0.9;
/** Right edge of the lot, towards Bankstraße. */
export const PARKING_RIGHT = -PARKING_INSET;
/** Front edge of the lot, towards Ulmenplatz. */
export const PARKING_FRONT = -PARKING_INSET;
/** Left end of the lot; keeps a small margin to the field border. */
export const PARKING_LEFT = -16;

/** Asphalt of the parking lot, kept inside the Frei-Parken field. */
function parkingArea(): string {
	return `M ${PARKING_LEFT} ${PARKING_BACK} L ${PARKING_RIGHT} ${PARKING_BACK} L ${PARKING_RIGHT} ${PARKING_FRONT} L ${PARKING_LEFT} ${PARKING_FRONT} Z`;
}

/** White bay lines: one divider per boundary plus the back line. */
function parkingMarks(): string {
	let d = '';
	for (let i = 0; i <= PARKING_BAYS; i++) {
		const x = round(PARKING_ROW_LEFT + i * PARKING_BAY_WIDTH);
		d += `M ${x} ${PARKING_KERB} L ${x} ${PARKING_BACK} `;
	}
	d += `M ${round(PARKING_ROW_LEFT)} ${PARKING_BACK} L ${round(PARKING_ROW_RIGHT)} ${PARKING_BACK}`;
	return d.trim();
}

/** Centre of a marked bay, in middle-area percentages. */
export function parkingBay(index: number): { x: number; y: number } {
	return {
		x: PARKING_ROW_LEFT + (index + 0.5) * PARKING_BAY_WIDTH,
		y: (PARKING_KERB + PARKING_BACK) / 2
	};
}

/** Point just inside the lot where a car leaves the ring, in middle-area
    percentages. Approaching it from the junction keeps the car on asphalt. */
export function parkingEntry(): { x: number; y: number } {
	return { x: PARKING_RIGHT, y: PARKING_FRONT };
}

const outer = areaPath();
const inner = innerPath();
const parking = parkingArea();

export const ROAD_PATHS = {
	/** Filled road surface: outer outline plus the inner hole. */
	area: `${outer} ${inner}`,
	outer,
	inner,
	ring: ringMarks(),
	spurs: spurMarks(),
	/** Asphalt of the parking lot, kept inside the Frei-Parken field. */
	parking,
	/** Marked bays of the parking lot. */
	parkingBays: parkingMarks()
};
