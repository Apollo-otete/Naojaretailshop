package ordering

import "testing"

// The Ordering context's vocabulary is pure functions today: the order-status string is validated
// here, while the payment-status labels are carried until the payment slice gives them rules.

func TestOrderStatusNamesAKnownState(t *testing.T) {
	for _, s := range []OrderStatus{
		StatusPendingPayment, StatusPaid, StatusProcessing, StatusReady,
		StatusDispatched, StatusDelivered, StatusCancelled, StatusFailed, StatusExpired,
	} {
		if !s.Valid() {
			t.Errorf("%q is not a valid order status", s)
		}
	}
	if (OrderStatus("not_a_status")).Valid() {
		t.Error("an unrecognised status was accepted")
	}
}
