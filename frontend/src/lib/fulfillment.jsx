const SATURDAY = 6;
export const FREE_DELIVERY_THRESHOLD = 600;

function getNow() {
  return new Date();
}

export function isPickupOpen() {
  return true;
}

export function isDeliveryOpen(now = getNow()) {
  return now.getDay() !== SATURDAY;
}

export function formatDeliveryMessage(now = getNow()) {
  if (!isDeliveryOpen(now)) {
    return `Online orders are accepted 24/7. Delivery is unavailable on Saturdays.`;
  }

  return `Online orders are accepted 24/7. Delivery is available Sunday-Friday.`;
}

export function formatPickupMessage() {
  return `Online orders are accepted 24/7. Pickup is available daily.`;
}

export function getDeliveryFee(subtotal) {
  return subtotal >= FREE_DELIVERY_THRESHOLD ? 0 : 150;
}

export function isFreeDelivery(subtotal) {
  return subtotal >= FREE_DELIVERY_THRESHOLD;
}

export const STORE_LOCATION = "Kakamega, Lurambi, Opposite Bamboo";

export const SHOP_HOURS = {
  online: "24/7",
  pickup: "Daily",
};

export const DELIVERY_HOURS = {
  sundayToFriday: "Available",
  saturday: "Unavailable",
};
