import { describe, expect, it } from 'vitest';
import {
	CORNERS,
	CORNER_ORDER,
	CORNER_RADIUS,
	PARKING_BAYS,
	PARKING_BAY_WIDTH,
	PARKING_CORNER,
	PARKING_FRONT,
	PARKING_INSET,
	PARKING_LEFT,
	PARKING_RIGHT,
	PARKING_ROW_LEFT,
	PARKING_ROW_RIGHT,
	driveDirection,
	junction,
	parkingBay,
	parksOnCorner,
	ROAD_PATHS,
	SPUR_REACH,
	spurEnd
} from './road';

describe('road geometry', () => {
	it('puts every junction on the ring centre line', () => {
		for (const key of CORNER_ORDER) {
			const corner = CORNERS[key];
			const point = junction(key);
			const radius = Math.hypot(point.x - corner.cx, point.y - corner.cy);
			expect(radius).toBeCloseTo(CORNER_RADIUS, 1);
		}
	});

	it('reaches the same distance into every corner field', () => {
		for (const key of CORNER_ORDER) {
			const corner = CORNERS[key];
			const end = spurEnd(key);
			// One step into the field, in the direction of the spur.
			expect(end.x).toBeCloseTo((corner.dx > 0 ? 100 : 0) + Math.sign(corner.dx) * SPUR_REACH, 5);
			expect(end.y).toBeCloseTo((corner.dy > 0 ? 100 : 0) + Math.sign(corner.dy) * SPUR_REACH, 5);
		}
	});

	it('builds a closed surface with two kerb fillets per corner', () => {
		expect(ROAD_PATHS.area).not.toContain('NaN');
		expect(ROAD_PATHS.outer.trimEnd().endsWith('Z')).toBe(true);
		expect(ROAD_PATHS.inner.trimEnd().endsWith('Z')).toBe(true);
		// Every corner opens into its spur through two fillets, the parking
		// lot included, so all four junctions look the same.
		expect(ROAD_PATHS.outer.match(/A 1\.2 1\.2 0 0 0/g)).toHaveLength(8);
	});

	it('opens the ring edge at the parking corner like every other spur', () => {
		// The driveway is part of the ring surface, so the kerb lines follow it
		// into the lot instead of stopping at the corner field.
		expect(ROAD_PATHS.outer).toContain('L -5.77 -2.23');
		expect(ROAD_PATHS.outer).toContain('A 2.5 2.5 0 0 1 -2.23 -5.77');
	});

	it('breaks the centre markings at the junctions', () => {
		expect(ROAD_PATHS.ring).not.toContain('NaN');
		expect(ROAD_PATHS.ring.match(/M/g)).toHaveLength(4);
		// The Frei-Parken spur runs into its parking lot, so it has no dash.
		expect(ROAD_PATHS.spurs.match(/M/g)).toHaveLength(3);
	});
});

describe('parking lot', () => {
	it('places every bay inside the Frei-Parken corner field', () => {
		// One corner field is a sixth of the middle area wide, plus the grid
		// gap, so the field edge never reaches -100/6.
		const edge = -100 / 6;
		expect(PARKING_CORNER).toBe(20);
		expect(PARKING_LEFT).toBeGreaterThan(edge);
		expect(PARKING_ROW_LEFT).toBeGreaterThan(PARKING_LEFT);
		expect(PARKING_ROW_RIGHT).toBeLessThan(PARKING_RIGHT);
		for (let bay = 0; bay < PARKING_BAYS; bay++) {
			const point = parkingBay(bay);
			expect(point.x).toBeGreaterThan(edge);
			expect(point.x).toBeLessThan(0);
			expect(point.y).toBeGreaterThan(edge);
			expect(point.y).toBeLessThan(0);
		}
	});

	it('keeps the lot clear of the field border and the neighbours', () => {
		expect(ROAD_PATHS.parking).not.toContain('NaN');
		// The grid gap and the 1px border need up to ~0.75 units at the
		// smallest board; the lot stays outside them.
		expect(PARKING_INSET).toBeGreaterThanOrEqual(0.8);
		expect(PARKING_RIGHT).toBe(-PARKING_INSET);
		expect(PARKING_FRONT).toBe(-PARKING_INSET);
		// Nothing reaches x=0 or y=0, the edges towards Bankstraße and
		// Ulmenplatz, so the asphalt never covers their borders.
		expect(PARKING_RIGHT).toBeLessThan(0);
		expect(PARKING_FRONT).toBeLessThan(0);
		expect(ROAD_PATHS.parking).not.toContain('L 0 0');
		expect(PARKING_ROW_RIGHT).toBeCloseTo(PARKING_ROW_LEFT + PARKING_BAYS * PARKING_BAY_WIDTH, 5);
	});

	it('paints one divider per bay boundary plus the back line', () => {
		expect(ROAD_PATHS.parkingBays).not.toContain('NaN');
		expect(ROAD_PATHS.parkingBays.match(/M/g)).toHaveLength(PARKING_BAYS + 2);
	});
});

describe('driveDirection', () => {
	it('drives forward for normal and advance moves, however far', () => {
		expect(driveDirection(5, 8, 'forward')).toBe(1);
		expect(driveDirection(2, 0, 'forward')).toBe(1);
		expect(driveDirection(14, 11, 'forward')).toBe(1);
	});

	it('drives backwards for the back cards', () => {
		expect(driveDirection(5, 2, 'backward')).toBe(-1);
		expect(driveDirection(0, 37, 'backward')).toBe(-1);
	});

	it('takes the shorter way to jail', () => {
		expect(driveDirection(2, 10, 'jail')).toBe(1);
		expect(driveDirection(20, 10, 'jail')).toBe(-1);
		expect(driveDirection(30, 10, 'jail')).toBe(-1);
		expect(driveDirection(39, 10, 'jail')).toBe(1);
	});
});

describe('parksOnCorner', () => {
	it('parks cars that finished their move on any corner field', () => {
		expect(parksOnCorner(0, 0)).toBe(true);
		expect(parksOnCorner(10, 10)).toBe(true);
		expect(parksOnCorner(20, 20)).toBe(true);
		expect(parksOnCorner(30, 30)).toBe(true);
	});

	it('keeps cars on the ring while they are still driving', () => {
		expect(parksOnCorner(9, 10)).toBe(false);
		expect(parksOnCorner(10, 12)).toBe(false);
	});
});
