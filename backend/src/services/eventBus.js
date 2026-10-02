// ==========================================
// SSE Event Bus for Admin Real-Time Feed
// ==========================================

class EventBus {
  constructor() {
    this.clients = new Set();
    this.eventHistory = [];
    this.maxHistory = 50;
  }

  /**
   * Register a new SSE client (Admin browser)
   */
  addClient(res) {
    this.clients.add(res);
    res.on('close', () => {
      this.clients.delete(res);
    });
  }

  /**
   * Broadcast an event to all connected admin clients and record in history
   */
  broadcast(eventType, payload) {
    const event = {
      id: Date.now().toString(),
      type: eventType,
      data: payload,
      timestamp: new Date().toISOString()
    };

    // Keep recent history for instant retrieval when admin opens page
    this.eventHistory.unshift(event);
    if (this.eventHistory.length > this.maxHistory) {
      this.eventHistory.pop();
    }

    const sseFormatted = `id: ${event.id}\nevent: ${eventType}\ndata: ${JSON.stringify(event)}\n\n`;

    this.clients.forEach(client => {
      try {
        client.write(sseFormatted);
      } catch (err) {
        this.clients.delete(client);
      }
    });
  }

  /**
   * Get latest recorded events
   */
  getRecentEvents(limit = 15) {
    return this.eventHistory.slice(0, limit);
  }

  /**
   * Get count of active admin sessions
   */
  getConnectedCount() {
    return this.clients.size;
  }
}

const eventBus = new EventBus();
module.exports = eventBus;
