const SATURDAY = 6;
export const FREE_DELIVERY_THRESHOLD = 600;
const DELIVERY_CUTOFF_HOUR = 16;

function getNow(): Date {
  return new Date();
}

export function isPickupOpen(now: Date = getNow()): boolean {
  const d = now.getDay();
  const h = now.getHours() + now.getMinutes() / 60;

  if (d === SATURDAY) return false;
  if (d >= 0 && d <= 4) return h >= 8 && h < 20;
  if (d === 5) return h >= 8 && h < 16;
  return false;
}

export function isDeliveryOpen(now: Date = getNow()): boolean {
  const d = now.getDay();
  const h = now.getHours() + now.getMinutes() / 60;

  if (d === SATURDAY) return false;
  return h >= 8 && h < 16;
}

export function formatDeliveryMessage(now: Date = getNow()): string {
  const d = now.getDay();
  const hour = now.getHours();

  if (d === SATURDAY) {
    return `We're closed on Saturdays. Delivery resumes Sunday 8:00am-4:00pm.`;
  }

  if (!isDeliveryOpen(now)) {
    return `Delivery hours are 8:00am-4:00pm (Sun-Fri).`;
  }

  if (hour < DELIVERY_CUTOFF_HOUR) {
    return `Order before 4:00pm for same-day delivery.`;
  }

  return `Orders after 4:00pm are delivered the next day.`;
}

export function formatPickupMessage(now: Date = getNow()): string {
  const d = now.getDay();

  if (d === SATURDAY) {
    return `Pickup is unavailable on Saturdays. Shop opens Sunday 8:00am. Your order will be ready 1-2 hours after opening.`;
  }

  if (isPickupOpen(now)) {
    return `Ready for pickup in 1-2 hours.`;
  }

  return `We're currently closed. Your order will be ready 1-2 hours after we open.`;
}

export function getDeliveryFee(subtotal: number): number {
  return subtotal >= FREE_DELIVERY_THRESHOLD ? 0 : 150;
}

export function isFreeDelivery(subtotal: number): boolean {
  return subtotal >= FREE_DELIVERY_THRESHOLD;
}

export const STORE_LOCATION = "Kakamega, Lurambi, Opposite Bamboo";

export const SHOP_HOURS = {
  sundayToThursday: "8am-8pm",
  friday: "8am-4pm",
  saturday: "Closed",
};

export const DELIVERY_HOURS = {
  sundayToFriday: "8am-4pm",
  saturday: "Closed",
};
