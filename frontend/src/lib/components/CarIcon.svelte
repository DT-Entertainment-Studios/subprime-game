<script lang="ts">
	let {
		color = 'var(--car-body-fallback)',
		initial = '',
		active = false
	}: {
		color?: string;
		initial?: string;
		active?: boolean;
	} = $props();

	// Unique per car so the lamp filter and beam gradient of the eight cars on
	// the board never collide.
	const uid = $props.id();
</script>

<!-- Top-down car, pointing up. The white outline keeps every player colour
     readable on the dark asphalt. The car on turn gets lit headlights plus a
     static headlight beam; every other car keeps dark glass. -->
<svg viewBox="0 0 30 52" data-lit={active ? 'true' : 'false'} style="--car: {color}" aria-hidden="true">
	{#if active}
		<defs>
			<linearGradient id={`${uid}-beam`} x1="0" y1="1" x2="0" y2="0">
				<stop offset="0" stop-color="var(--car-beam)" stop-opacity="0.5" />
				<stop offset="1" stop-color="var(--car-beam)" stop-opacity="0" />
			</linearGradient>
			<filter id={`${uid}-lamp`} x="-300%" y="-300%" width="700%" height="700%">
				<feGaussianBlur stdDeviation="1.1" result="b" />
				<feMerge>
					<feMergeNode in="b" />
					<feMergeNode in="b" />
					<feMergeNode in="SourceGraphic" />
				</feMerge>
			</filter>
		</defs>
		<path data-beam d="M 9 2 L 21 2 L 28 -30 L 2 -30 Z" fill={`url(#${uid}-beam)`} />
	{/if}

	<ellipse cx="15" cy="28" rx="12" ry="21" fill="var(--car-shadow)" />
	<rect x="1" y="10" width="5" height="11" rx="2.2" fill="var(--car-tyre)" />
	<rect x="24" y="10" width="5" height="11" rx="2.2" fill="var(--car-tyre)" />
	<rect x="1" y="31" width="5" height="11" rx="2.2" fill="var(--car-tyre)" />
	<rect x="24" y="31" width="5" height="11" rx="2.2" fill="var(--car-tyre)" />

	<rect
		x="5"
		y="3"
		width="20"
		height="46"
		rx="9"
		fill="var(--car)"
		stroke="var(--car-body-stroke)"
		stroke-width="2.2"
	/>

	<path d="M9 13 Q15 9 21 13 L20 20 Q15 23 10 20 Z" fill="var(--car-windscreen)" />
	<path
		d="M10 34 Q15 32 20 34 L19 39 Q15 41 11 39 Z"
		fill="var(--car-rear-window)"
		opacity="0.95"
	/>
	<rect x="10" y="21.5" width="10" height="11" rx="3" fill="var(--car-cabin)" />

	{#if initial}
		<text
			x="15"
			y="29.4"
			text-anchor="middle"
			font-size="8"
			font-weight="800"
			fill="var(--car-initial)"
			stroke="var(--car-initial-stroke)"
			stroke-width="1.6"
			paint-order="stroke"
		>
			{initial}
		</text>
	{/if}

	<rect
		x="8.2"
		y="5.5"
		width="3.6"
		height="2"
		rx="1"
		fill={active ? 'var(--car-lamp-on)' : 'var(--car-lamp-off)'}
		filter={active ? `url(#${uid}-lamp)` : undefined}
	/>
	<rect
		x="18.2"
		y="5.5"
		width="3.6"
		height="2"
		rx="1"
		fill={active ? 'var(--car-lamp-on)' : 'var(--car-lamp-off)'}
		filter={active ? `url(#${uid}-lamp)` : undefined}
	/>
	<rect x="7" y="46" width="4.2" height="2.4" rx="1" fill="var(--car-taillight)" />
	<rect x="18.8" y="46" width="4.2" height="2.4" rx="1" fill="var(--car-taillight)" />
</svg>

<style>
	svg {
		display: block;
		width: 100%;
		height: auto;
		/* The beam reaches above the nose; without this the SVG edge clips it. */
		overflow: visible;
	}
</style>
