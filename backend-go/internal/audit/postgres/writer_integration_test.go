//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/audit"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/dbtest"
	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/observability"
)

// The audit log is the one table written from every sensitive path, so what deserves an
// integration test here is what a fake cannot testify to: that the jsonb columns hold the JSON
// that was handed over, that the correlation id reaches the row, that an entry commits and rolls
// back with the change it describes (ERD T-10), and that the append-only trigger actually fires.

const (
	// refHash is the Argon2id reference vector from internal/password/argon2id_test.go. The
	// users table CHECK-constrains the column, so a seeded actor needs a well-formed hash.
	refHash = "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$2x8WSIXNLdCDSjaiBZgpg8B9cq2w1ir2HdFIAe46OtU"

	corrID = "0f6c1f2a-7e0b-4a1d-9f3c-5d2b8a1c4e77"
)

type fixture struct {
	pool   *pgxpool.Pool
	writer *Writer
	tx     *database.TxRunner
	userID int64
	now    time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()

	pool := dbtest.Pool(t)
	// identity.users cascades to sessions, grants and the audit log's actor, so this one TRUNCATE
	// empties everything this package writes — and RESTART IDENTITY is what makes "the actor is
	// user 1" an assertion rather than a hope.
	dbtest.Truncate(t, pool, "identity.users")

	w, err := New(pool)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var id int64
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO identity.users (email, full_name, password_hash, account_kind)
		VALUES ('auditor@mail.test', 'Audit Subject', $1, 'admin')
		RETURNING id`, refHash).Scan(&id); err != nil {
		t.Fatalf("seeding an actor: %v", err)
	}

	return &fixture{pool: pool, writer: w, tx: database.NewTxRunner(pool), userID: id,
		now: time.Now().UTC().Truncate(time.Microsecond)}
}

// exec is the only correct way to write inside a unit of work in these tests: f.pool.Exec would
// take a second connection and commit beside the transaction rather than inside it, which is
// precisely the mistake T-10 is about.
func (f *fixture) exec(ctx context.Context, t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := database.DB(ctx, f.pool).Exec(ctx, query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

func (f *fixture) scalarInt(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("scalar %q: %v", query, err)
	}
	return n
}

func (f *fixture) scalarString(t *testing.T, query string, args ...any) string {
	t.Helper()
	var s string
	if err := f.pool.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("scalar %q: %v", query, err)
	}
	return s
}

// request carries the correlation id the transport would have put there, because §10 asks for the
// same id in the log line, the response and the audit row.
func (f *fixture) request() context.Context {
	return observability.WithRequestID(context.Background(), corrID)
}

func TestRecordWritesTheColumnsTheMatrixDeclaresMandatory(t *testing.T) {
	f := setup(t)
	resource := int64(42)

	e := audit.ByUser(f.userID, "catalogue.products.update_price", "product", &resource)
	e.PreviousValue = map[string]any{"price_cents": 150000}
	e.NewValue = map[string]any{"price_cents": 145000}
	e.Reason = "supplier price cut, ADR-016"
	e.IP = "203.0.113.7"
	e.OccurredAt = f.now

	if err := f.writer.Record(f.request(), e); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var (
		actorID                 *int64
		kind, action, resType   string
		resID                   *int64
		previous, next          json.RawMessage
		reason, ip, correlation string
		occurred                time.Time
	)
	if err := f.pool.QueryRow(context.Background(), `
		SELECT actor_id, actor_kind, action, resource_type, resource_id,
		       previous_value, new_value, reason, host(ip), correlation_id, occurred_at
		FROM administration.audit_logs`).
		Scan(&actorID, &kind, &action, &resType, &resID,
			&previous, &next, &reason, &ip, &correlation, &occurred); err != nil {
		t.Fatalf("reading the entry back: %v", err)
	}

	if actorID == nil || *actorID != f.userID {
		t.Errorf("actor_id = %v, want %d", actorID, f.userID)
	}
	if kind != "user" || action != "catalogue.products.update_price" {
		t.Errorf("actor_kind/action = %q / %q", kind, action)
	}
	if resType != "product" || resID == nil || *resID != resource {
		t.Errorf("resource = %q %v, want product 42", resType, resID)
	}
	// jsonb reformats on the way in — a space after each colon, its own key order — so the
	// assertion is about the parsed content. What matters is that 150000 arrived as 150000.
	if got := cents(t, previous); got != 150000 {
		t.Errorf("previous_value = %s, want price_cents 150000", previous)
	}
	if got := cents(t, next); got != 145000 {
		t.Errorf("new_value = %s, want price_cents 145000", next)
	}
	if reason != e.Reason {
		t.Errorf("reason = %q, want %q", reason, e.Reason)
	}
	if ip != "203.0.113.7" {
		t.Errorf("ip = %q, want the address stored as an address", ip)
	}
	// The correlation id is the link between this row and the request the customer was given
	// (§10). Without it the audit log cannot be opened from a support ticket.
	if correlation != corrID {
		t.Errorf("correlation_id = %q, want %q", correlation, corrID)
	}
	if !occurred.Equal(f.now) {
		t.Errorf("occurred_at = %v, want the caller's %v", occurred, f.now)
	}
}

func TestTheActorlessKindsAreStoredAsTheSchemaAllows(t *testing.T) {
	f := setup(t)

	// A background job and a provider callback have no user to name. Inventing one would
	// attribute machine behaviour to a person.
	order := int64(7)
	if err := f.writer.Record(context.Background(),
		audit.SystemAction("inventory.reservations.expire", "order", &order)); err != nil {
		t.Fatalf("SystemAction: %v", err)
	}
	if err := f.writer.Record(context.Background(),
		audit.ProviderAction("payments.daraja.callback", "payment", nil)); err != nil {
		t.Fatalf("ProviderAction: %v", err)
	}

	rows, err := f.pool.Query(context.Background(), `
		SELECT actor_kind, action, resource_type, resource_id, reason, ip, correlation_id
		FROM administration.audit_logs ORDER BY id`)
	if err != nil {
		t.Fatalf("querying: %v", err)
	}
	defer rows.Close()

	type got struct {
		kind, action, resType   string
		resID                   *int64
		reason, ip, correlation *string
	}
	var out []got
	for rows.Next() {
		var g got
		if err := rows.Scan(&g.kind, &g.action, &g.resType, &g.resID, &g.reason, &g.ip, &g.correlation); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading rows: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("%d rows, want 2: %+v", len(out), out)
	}
	if out[0].kind != "system" || out[1].kind != "provider" {
		t.Errorf("actor kinds = %q, %q", out[0].kind, out[1].kind)
	}
	if out[0].resID == nil || *out[0].resID != order {
		t.Errorf("system row resource_id = %v, want 7", out[0].resID)
	}
	if out[1].resID != nil {
		t.Errorf("provider row resource_id = %v, want NULL", *out[1].resID)
	}
	// An empty string in a nullable column would make "not given" and "given as empty" one row,
	// so the adapter turns both into SQL NULL.
	for i, g := range out {
		if g.reason != nil {
			t.Errorf("row %d: reason = %q, want NULL", i, *g.reason)
		}
		if g.ip != nil {
			t.Errorf("row %d: ip = %q, want NULL", i, *g.ip)
		}
		if g.correlation != nil {
			t.Errorf("row %d: correlation_id = %q, want NULL outside a request", i, *g.correlation)
		}
	}
}

func TestUnattributableEntriesAreRefused(t *testing.T) {
	f := setup(t)

	id := f.userID
	cases := []audit.Entry{
		{ActorKind: audit.KindUser, Action: "catalogue.products.update"},
		{ActorKind: audit.KindSystem},
		{ActorKind: audit.Kind("robot"), Action: "catalogue.products.update"},
		{ActorID: &id, ActorKind: audit.KindSystem, Action: "catalogue.products.update"},
		{ActorID: &id, ActorKind: audit.KindProvider, Action: "catalogue.products.update"},
	}
	for i, e := range cases {
		if err := f.writer.Record(context.Background(), e); !errors.Is(err, audit.ErrInvalidEntry) {
			t.Errorf("case %d (%+v): got %v, want ErrInvalidEntry", i, e, err)
		}
	}

	// A value that cannot be serialised is refused too, and it must not leave half a row behind:
	// a partial entry is what an auditor reads as a complete answer.
	halfWritten := audit.SystemAction("engagement.reviews.moderate", "review", nil)
	halfWritten.NewValue = make(chan int)
	if err := f.writer.Record(context.Background(), halfWritten); err == nil {
		t.Error("an unserialisable new_value was accepted")
	} else if errors.Is(err, audit.ErrInvalidEntry) {
		t.Errorf("a serialisation failure was reported as %v, want the marshal error beneath it", err)
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM administration.audit_logs`); got != 0 {
		t.Errorf("%d rows survived entries that were refused", got)
	}
}

func TestJSONNullAndAbsentAreNotTheSameRow(t *testing.T) {
	f := setup(t)

	var absent *int // a typed nil: the caller had no previous value, which is not the same as a null
	e := audit.ByUser(f.userID, "catalogue.products.archive", "product", nil)
	e.PreviousValue = absent
	e.NewValue = map[string]any{"cleared_by": nil}

	if err := f.writer.Record(context.Background(), e); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var previous, next *string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT previous_value, new_value FROM administration.audit_logs`).
		Scan(&previous, &next); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if previous != nil {
		t.Errorf("previous_value = %s, want SQL NULL for an absent value", *previous)
	}
	if next == nil {
		t.Error("new_value disappeared; the inner null is a value, not an absence")
	} else {
		var doc map[string]any
		if err := json.Unmarshal([]byte(*next), &doc); err != nil {
			t.Fatalf("new_value is not the jsonb it claims to be: %v (%s)", err, *next)
		}
		// The key is present and its value is null — the distinction between "this field was
		// cleared" and "no field was reported", which an auditor reads differently.
		value, present := doc["cleared_by"]
		if !present || value != nil {
			t.Errorf("new_value = %s, want cleared_by present and null", *next)
		}
	}
}

// ERD T-10 at the adapter level: the entry and the change share one transaction, so an entry
// cannot be lost while the change succeeds — and, the direction a fake never checks, an entry
// cannot survive a change that failed.
func TestTheAuditEntryCommitsAndRollsBackWithTheChange(t *testing.T) {
	f := setup(t)

	commit := f.tx.RunInTx(context.Background(), func(ctx context.Context) error {
		f.exec(ctx, t, `
			INSERT INTO identity.users (email, full_name, password_hash, account_kind)
			VALUES ('suspended@mail.test', 'Suspension Subject', $1, 'customer')`, refHash)
		return f.writer.Record(ctx, audit.ByUser(f.userID, "identity.customers.suspend", "user", nil))
	})
	if commit != nil {
		t.Fatalf("committing change + audit: %v", commit)
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM administration.audit_logs WHERE action = 'identity.customers.suspend'`); got != 1 {
		t.Errorf("%d audit rows for a change that committed, want 1", got)
	}

	rollback := f.tx.RunInTx(context.Background(), func(ctx context.Context) error {
		if err := f.writer.Record(ctx, audit.ByUser(f.userID, "identity.admins.manage", "user", nil)); err != nil {
			return err
		}
		return errors.New("the change failed")
	})
	if rollback == nil {
		t.Fatal("the rolled-back unit of work reported success")
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM administration.audit_logs WHERE action = 'identity.admins.manage'`); got != 0 {
		t.Errorf("%d audit rows for a change that failed", got)
	}
}

// The schema, not the repository, owns append-only. If these stop erroring, every promise the
// matrix makes about tamper evidence is false.
func TestHistoryCannotBeRewrittenOrDeleted(t *testing.T) {
	f := setup(t)

	e := audit.ByUser(f.userID, "administration.system_settings.manage", "setting", nil)
	e.NewValue = map[string]any{"key": "delivery_standard_fee_cents"}
	if err := f.writer.Record(context.Background(), e); err != nil {
		t.Fatalf("Record: %v", err)
	}

	for _, stmt := range []string{
		`UPDATE administration.audit_logs SET action = 'nothing.happened' WHERE true`,
		`DELETE FROM administration.audit_logs WHERE true`,
	} {
		_, err := f.pool.Exec(context.Background(), stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Errorf("%s was accepted (err = %v)", stmt, err)
			continue
		}
		// 42501 insufficient_privilege is what identity.refuse_audit_mutation raises.
		if pgErr.Code != "42501" {
			t.Errorf("%s failed with %s: %s, want 42501", stmt, pgErr.Code, pgErr.Message)
		}
	}

	if got := f.scalarString(t, `SELECT action FROM administration.audit_logs`); got != e.Action {
		t.Errorf("action = %q, want the original %q", got, e.Action)
	}

	// Append-only is not write-refusing: a test that only proved the trigger fires would also
	// pass if the table had become unwritable.
	if err := f.writer.Record(context.Background(),
		audit.SystemAction("administration.platform_health.read", "system", nil)); err != nil {
		t.Fatalf("a second append failed: %v", err)
	}
	if got := f.scalarInt(t, `SELECT count(*) FROM administration.audit_logs`); got != 2 {
		t.Errorf("%d rows, want 2", got)
	}
}

func TestAZeroTimestampMeansNowAndNotNothing(t *testing.T) {
	f := setup(t)

	before := time.Now().Add(-time.Second)
	if err := f.writer.Record(context.Background(),
		audit.SystemAction("engagement.inquiries.respond", "inquiry", nil)); err != nil {
		t.Fatalf("Record: %v", err)
	}
	var occurred time.Time
	if err := f.pool.QueryRow(context.Background(),
		`SELECT occurred_at FROM administration.audit_logs`).Scan(&occurred); err != nil {
		t.Fatalf("reading occurred_at: %v", err)
	}
	if occurred.Before(before) || occurred.After(time.Now().Add(time.Minute)) {
		t.Errorf("occurred_at = %v, want the present rather than the zero value", occurred)
	}
}

// New refuses a nil pool rather than letting the first privileged write panic after its change
// has already committed.
func TestNewRefusesAPoolThatIsNotThere(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Error("New accepted a nil pool")
	}
}

// cents reads the one field a price-change audit entry exists to record, and fails loudly if the
// column holds anything else: a whole jsonb document that will not parse is how a hand-written
// value gets mistaken for the adapter's.
func cents(t *testing.T, raw json.RawMessage) int64 {
	t.Helper()

	var doc map[string]int64
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s is not a JSON object of integers: %v", raw, err)
	}
	if len(doc) != 1 {
		t.Fatalf("%s holds %d fields, want the one that changed", raw, len(doc))
	}
	n, ok := doc["price_cents"]
	if !ok {
		t.Fatalf("%s has no price_cents", raw)
	}
	return n
}
