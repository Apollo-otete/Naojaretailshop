//go:build integration

package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Apollo-otete/Naojaretailshop/backend-go/internal/database"
)

// ADR-024's ownership gate is data before it is code: a key is narrowed because the catalogue says
// so. These tests read that statement back from a migrated server, because a column the adapter
// does not select and a column nobody populates look identical from a unit test.

// The same two predicates 0004's DO block raises on. Duplicated rather than referenced because
// the migration runs once, at a moment when nothing has drifted yet — the value here is that the
// shape of the question is provably answerable, and that the answer flips when the data does.
const (
	documentedButUndeclared = `
		SELECT count(*) FROM identity.permissions
		 WHERE position('[own]' in description) > 0 AND conditions = '{}'`
	declaredButUndocumented = `
		SELECT count(*) FROM identity.permissions
		 WHERE conditions <> '{}' AND position('[own]' in description) = 0`
)

func TestGrantsForCarriesTheConditionDeclaredForEachScopedKey(t *testing.T) {
	f := setup(t)
	user := f.insertUser(t, "scoped.reader@mail.test", "+254700000301")
	f.query(t, `INSERT INTO identity.user_roles (user_id, role_id)
		SELECT $1, id FROM identity.roles WHERE key = 'customer'`, user.ID)

	grants, err := f.store.GrantsFor(bgCtx(), user.ID)
	if err != nil {
		t.Fatalf("GrantsFor: %v", err)
	}

	if got := grants.Conditions["ordering.orders.read"]; !reflect.DeepEqual(got, []string{"owner_is_caller"}) {
		t.Errorf("Conditions[ordering.orders.read] = %v, want [owner_is_caller]", got)
	}

	// The absence matters as much as the presence: a caller who reads the published catalogue is
	// not being narrowed, and an evaluator that treated "no entry" as "some entry" would refuse
	// every unscoped key.
	if _, narrowed := grants.Conditions["catalogue.products.read.published"]; narrowed {
		t.Error("an unscoped permission arrived with a condition")
	}

	// And every condition returned must belong to a permission that was actually granted, or the
	// map is describing a policy the caller does not hold.
	for key := range grants.Conditions {
		if !has(grants.Permissions, key) {
			t.Errorf("Conditions carries %q, which is not in Permissions", key)
		}
	}

	// The count is taken from the catalogue rather than written here: what is being asserted is
	// that the adapter's map and the adapter's permission list describe the same policy.
	var scoped int
	for _, perm := range grants.Permissions {
		if len(grants.Conditions[perm]) > 0 {
			scoped++
		}
	}
	if want := f.scalarInt(t, `
		SELECT count(*) FROM identity.permissions
		 WHERE conditions <> '{}' AND key = ANY($1)`, grants.Permissions); scoped != int(want) {
		t.Errorf("%d of this caller's grants are narrowed, the catalogue says %d", scoped, want)
	}
	if scoped == 0 {
		t.Error("a customer's grants carry no conditions at all, so the ownership gate is not being read")
	}
}

func TestTheCatalogueRefusesAConditionNothingImplements(t *testing.T) {
	f := setup(t)

	// The direction that would widen access: data naming a rule Go does not have. The evaluator
	// refuses it too, but a database that will not store it cannot be shipped, which is the
	// cheaper place to find out.
	_, err := f.pool.Exec(bgCtx(), `
		UPDATE identity.permissions SET conditions = ARRAY['owner_is_the_moon']
		 WHERE key = 'ordering.orders.read'`)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("an unimplemented condition name was accepted: %v", err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != "permissions_conditions_are_implemented" {
		t.Errorf("code = %s constraint = %q, want 23514 permissions_conditions_are_implemented",
			pgErr.Code, pgErr.ConstraintName)
	}

	// The other direction is the one a test can say about this constraint: it is a whitelist, so
	// an empty array is always allowed and a permission is unscoped unless declared otherwise.
	if _, err := f.pool.Exec(bgCtx(), `
		UPDATE identity.permissions SET conditions = '{}' WHERE key = 'ordering.orders.read'`); err != nil {
		t.Fatalf("clearing a condition should be a plain update: %v", err)
	}
	f.query(t, `
		UPDATE identity.permissions SET conditions = ARRAY['owner_is_caller']
		 WHERE key = 'ordering.orders.read'`)
}

func TestTheOwnershipDeclarationMatchesTheProseInBothDirections(t *testing.T) {
	f := setup(t)

	if n := f.scalarInt(t, documentedButUndeclared); n != 0 {
		t.Errorf("%d permissions are described as [own] without declaring owner_is_caller, so the"+
			" ownership rule is prose only", n)
	}
	if n := f.scalarInt(t, declaredButUndocumented); n != 0 {
		t.Errorf("%d permissions declare a condition nobody asked for in the matrix", n)
	}
	if n := f.scalarInt(t, `SELECT count(*) FROM identity.permissions WHERE conditions <> '{}'`); n != 15 {
		t.Errorf("%d permissions carry a condition, want 15 (matrix §3 marks fifteen keys [own])", n)
	}

	// The assertion above has to be shown to be capable of failing. This is the drift it refuses:
	// a description edited to claim ownership, its condition left empty — which is exactly how a
	// `[own]` key ends up enforced by nobody. Everything happens inside a transaction that is
	// failed on purpose, so the live catalogue is left as migrated.
	drift := errors.New("simulated drift")
	err := f.tx.RunInTx(bgCtx(), func(ctx context.Context) error {
		if _, err := database.DB(ctx, f.pool).Exec(ctx, `
			UPDATE identity.permissions
			   SET description = description || ' [own]'
			 WHERE key = 'catalogue.products.read.published'`); err != nil {
			return err
		}
		var n int64
		if err := database.DB(ctx, f.pool).QueryRow(ctx, documentedButUndeclared).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("the drift query found %d mismatches inside the transaction, want 1 — 0004's"+
				" assertion is comparing something that cannot change", n)
		}
		return drift
	})
	if !errors.Is(err, drift) {
		t.Fatalf("RunInTx should have rolled the drift back: %v", err)
	}
	if n := f.scalarInt(t, documentedButUndeclared); n != 0 {
		t.Error("the failed transaction left its drift behind")
	}
	if d := f.scalarString(t,
		`SELECT description FROM identity.permissions WHERE key = 'catalogue.products.read.published'`); strings.Contains(d, "[own]") {
		t.Error("the description edit survived a rolled-back transaction")
	}
}
