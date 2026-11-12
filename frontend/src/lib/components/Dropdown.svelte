<script lang="ts">
	interface DropdownOption {
		value: string | number;
		label: string;
		color?: string;
	}

	let {
		id,
		options,
		value = $bindable(),
		onselect,
		ariaLabel
	}: {
		id: string;
		options: DropdownOption[];
		value?: string | number;
		onselect?: () => void;
		ariaLabel?: string;
	} = $props();

	let open = $state(false);
	let active = $state(0);
	let root: HTMLElement | undefined = $state();
	let trigger: HTMLButtonElement | undefined = $state();

	const selected = $derived(options.find((o) => o.value === value));

	function openMenu() {
		const current = options.findIndex((o) => o.value === value);
		active = current >= 0 ? current : 0;
		open = true;
	}

	function move(step: number, destination?: 'first' | 'last') {
		if (destination === 'first') active = 0;
		else if (destination === 'last') active = options.length - 1;
		else active = Math.min(options.length - 1, Math.max(0, active + step));
	}

	function pick(option: DropdownOption) {
		value = option.value;
		onselect?.();
		open = false;
		trigger?.focus();
	}

	function onKeydown(event: KeyboardEvent) {
		switch (event.key) {
			case 'Enter':
			case ' ':
				event.preventDefault();
				if (open) pick(options[active]);
				else openMenu();
				break;
			case 'ArrowDown':
				event.preventDefault();
				if (open) move(1);
				else openMenu();
				break;
			case 'ArrowUp':
				event.preventDefault();
				if (open) move(-1);
				else openMenu();
				break;
			case 'Home':
				if (open) {
					event.preventDefault();
					move(0, 'first');
				}
				break;
			case 'End':
				if (open) {
					event.preventDefault();
					move(0, 'last');
				}
				break;
			case 'Escape':
			case 'Tab':
				open = false;
				break;
		}
	}

	function closeIfOutside(event: PointerEvent) {
		if (root && !root.contains(event.target as Node)) open = false;
	}

	$effect(() => {
		document.addEventListener('pointerdown', closeIfOutside);
		return () => document.removeEventListener('pointerdown', closeIfOutside);
	});
</script>

<div class="dropdown" bind:this={root}>
	<button
		class="trigger"
		{id}
		type="button"
		role="combobox"
		aria-controls={open ? `${id}-menu` : undefined}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-label={ariaLabel}
		aria-activedescendant={open ? `${id}-opt-${active}` : undefined}
		bind:this={trigger}
		onclick={() => (open ? (open = false) : openMenu())}
		onkeydown={onKeydown}
	>
		{#if selected?.color}<span class="dot" style="background: {selected.color}"></span>{/if}
		<span class="label">{selected?.label ?? '–'}</span>
		<span class="chevron" aria-hidden="true">▾</span>
	</button>

	{#if open}
		<div class="menu" id={`${id}-menu`} role="listbox" aria-labelledby={id}>
			{#each options as option, i (option.value)}
				<button
					id={`${id}-opt-${i}`}
					type="button"
					class="option"
					class:active={i === active}
					role="option"
					aria-selected={option.value === value}
					tabindex="-1"
					onpointerdown={(e) => e.preventDefault()}
					onmousemove={() => (active = i)}
					onclick={() => pick(option)}
				>
					{#if option.color}<span class="dot" style="background: {option.color}"></span>{/if}
					<span class="label">{option.label}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.dropdown {
		position: relative;
	}

	.trigger {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
		padding: 0.45rem 0.6rem;
		background: var(--md-sys-color-surface);
		color: var(--md-sys-color-on-surface);
		border: 1px solid var(--md-sys-color-outline);
		border-radius: var(--md-sys-shape-corner-small);
		font-family: inherit;
		font-size: 16px; /* body-large, tokens.css:171 */
		line-height: 24px;
		text-align: left;
		cursor: pointer;
	}

	.trigger:focus-visible {
		outline: var(--cd-focus-ring-width) solid var(--cd-focus-ring-color);
		outline-offset: var(--cd-focus-ring-offset);
	}

	.label {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.chevron {
		font-size: 0.8rem;
		opacity: 0.7;
	}

	.menu {
		position: absolute;
		top: calc(100% + 2px);
		left: 0;
		right: 0;
		z-index: 30;
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		padding: 0.3rem;
		max-height: 16rem;
		overflow-y: auto;
		background: var(--md-sys-color-surface-container-high);
		border: 1px solid var(--md-sys-color-outline-variant);
		border-radius: var(--md-sys-shape-corner-small);
		box-shadow: 0 var(--md-sys-elevation-2) calc(var(--md-sys-elevation-5))
			color-mix(in srgb, var(--md-sys-color-shadow) 22%, transparent);
	}

	.option {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
		padding: 0.4rem 0.5rem;
		background: transparent;
		color: var(--md-sys-color-on-surface);
		border: 0;
		border-radius: var(--md-sys-shape-corner-extra-small);
		font-family: inherit;
		font-size: 0.95rem;
		text-align: left;
		cursor: pointer;
	}

	.option.active {
		background: var(--md-sys-color-secondary-container);
		color: var(--md-sys-color-on-secondary-container);
	}

	.option:focus-visible {
		outline: var(--cd-focus-ring-width) solid var(--cd-focus-ring-color);
		outline-offset: var(--cd-focus-ring-offset);
	}

	.dot {
		flex: none;
		width: 0.9rem;
		height: 0.9rem;
		border-radius: 50%;
		border: 1px solid var(--cd-ral9016);
	}
</style>
