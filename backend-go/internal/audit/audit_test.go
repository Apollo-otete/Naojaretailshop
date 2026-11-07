package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
)

func int64p(v int64) *int64 { return &v }

func TestByUserAndSystemActionsAreRecordableAsBuilt(t *testing.T) {
	cases := []struct {
		name string
		e    audit.Entry
	}{
		{"by user", audit.ByUser(9, "catalogue.products.update_price", "product", int64p(3))},
		{"system", audit.SystemAction("inventory.reservations.expired", "reservation", int64p(1))},
		{"provider", audit.ProviderAction("payments.callback.applied", "payment", int64p(2))},
	}
	for _, c := range cases {
		if err := c.e.Validate(); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// The two mistakes that would produce an audit log nobody can read back: an action with no
// name, and an entry that claims a user acted but cannot say which one.
func TestValidateRefusesUnattributableEntries(t *testing.T) {
	cases := []struct {
		name string
		e    audit.Entry
	}{
		{"no action", audit.Entry{ActorKind: audit.KindUser, ActorID: int64p(1)}},
		{"user with no id", audit.Entry{ActorKind: audit.KindUser, Action: "x"}},
		{"system with an id", audit.Entry{ActorKind: audit.KindSystem, Action: "x", ActorID: int64p(1)}},
		{"unknown kind", audit.Entry{ActorKind: "bot", Action: "x"}},
		{"empty kind", audit.Entry{Action: "x"}},
	}
	for _, c := range cases {
		err := c.e.Validate()
		if !errors.Is(err, audit.ErrInvalidEntry) {
			t.Errorf("%s: Validate = %v, want ErrInvalidEntry", c.name, err)
		}
	}
}

// A price change is the case the matrix insists on: previous value, new value, reason, actor and
// timestamp, all in one row. If the record cannot carry them it will be reconstructed from logs.
func TestEntryCarriesTheFieldsTheMatrixDeclaresMandatory(t *testing.T) {
	now := time.Now()
	user := int64p(4)
	e := audit.ByUser(*user, "catalogue.products.update_price", "product", int64p(77))
	e.PreviousValue = map[string]any{"price_cents": 450000}
	e.NewValue = map[string]any{"price_cents": 390000}
	e.Reason = "Supplier cost reduction, approved by the owner on WhatsApp."
	e.IP = "196.189.10.4"
	e.OccurredAt = now

	if err := e.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("the adapter sends this to a JSONB column: %v", err)
	}
	for _, want := range []string{"450000", "390000", "Supplier cost reduction", "196.189.10.4", "update_price"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the record dropped %q: %s", want, body)
		}
	}
}

// The port is what the service compiles against; if the adapter drifts, this fails at build.
var (
	_ audit.Writer = funcWriter(func(context.Context, audit.Entry) error { return nil })
	_ audit.Writer = (*countingWriter)(nil)
)

type funcWriter func(context.Context, audit.Entry) error

func (f funcWriter) Record(ctx context.Context, e audit.Entry) error { return f(ctx, e) }

type countingWriter struct{ entries []audit.Entry }

func (c *countingWriter) Record(_ context.Context, e audit.Entry) error {
	c.entries = append(c.entries, e)
	return nil
}

func TestWriterPortIsSatisfiable(t *testing.T) {
	var w audit.Writer = &countingWriter{}
	if err := w.Record(context.Background(), audit.SystemAction("jobs ran", "system", nil)); err != nil {
		t.Fatal(err)
	}
	if len(w.(*countingWriter).entries) != 1 {
		t.Fatal("the port did not receive the entry")
	}
}
