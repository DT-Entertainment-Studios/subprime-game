// Shared trade UI state: popups and the trade panel talk to each other.

class TradeUI {
	/** Whether the "Handel" details panel is opened. */
	panelOpen = $state(false);
	/** Id of an open offer the trade panel should prefill as counter-offer. */
	counterRespond = $state<string | null>(null);

	/**
	 * Requests the trade panel to open and prefill a counter-offer for the
	 * given offer id. The panel consumes and clears the request.
	 */
	requestCounter(tradeId: string): void {
		this.counterRespond = tradeId;
		this.panelOpen = true;
	}
}

export const tradeUI = new TradeUI();
