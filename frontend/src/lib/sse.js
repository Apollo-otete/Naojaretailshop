// ==========================================================
// Server-Sent Events (SSE) Client with Auto-Reconnect
// ==========================================================

export class SSEClient {
  constructor(url, onEvent, onError) {
    this.url = url;
    this.onEvent = onEvent;
    this.onError = onError;
    this.source = null;
    this.reconnectTimeout = null;
    this.isConnected = false;
  }

  connect() {
    if (this.source) {
      this.disconnect();
    }

    try {
      this.source = new EventSource(this.url);

      this.source.onopen = () => {
        this.isConnected = true;
      };

      this.source.onmessage = (e) => {
        try {
          const data = JSON.parse(e.data);
          if (this.onEvent) this.onEvent(data);
        } catch (err) {
          console.warn('Failed to parse SSE message:', err);
        }
      };

      // Custom event types
      const eventTypes = ['order_created', 'payment_received', 'payment_failed', 'low_stock', 'review_submitted', 'connected'];
      eventTypes.forEach(type => {
        this.source.addEventListener(type, (e) => {
          try {
            const data = JSON.parse(e.data);
            if (this.onEvent) this.onEvent(data);
          } catch (err) {
            console.warn(`Failed to parse SSE event [${type}]:`, err);
          }
        });
      });

      this.source.onerror = (err) => {
        this.isConnected = false;
        if (this.onError) this.onError(err);
        this.source.close();
        this.source = null;

        // Auto reconnect after 5 seconds
        clearTimeout(this.reconnectTimeout);
        this.reconnectTimeout = setTimeout(() => {
          this.connect();
        }, 5000);
      };
    } catch (err) {
      console.warn('SSE initialization failed:', err);
    }
  }

  disconnect() {
    clearTimeout(this.reconnectTimeout);
    if (this.source) {
      this.source.close();
      this.source = null;
    }
    this.isConnected = false;
  }
}
