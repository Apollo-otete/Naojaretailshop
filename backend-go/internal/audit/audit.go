// Package audit is the Administration & Audit context's write side: the one append-only record
// of "who changed what, and why".
//
// ERD §5.9 and T-10 make two things non-negotiable, and this package exists so they cannot be
// done by discipline. The first is that a privileged write and its audit entry share one
// transaction — an audit entry that can be lost while the change succeeds is not an audit trail.
// The second is that the table refuses UPDATE and DELETE at the database, so "append-only" is a
// property of the schema rather than of everyone remembering to only INSERT.
package audit

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Kind is constrained by administration.audit_logs' CHECK. NULL actor_id and actor_kind
// 'system' is the shape of "the reservation sweep ran"; 'provider' is Daraja. Both are events an
// operator needs to see, which is why the actor column is not simply the acting user.
type Kind string

const (
	KindUser     Kind = "user"
	KindSystem   Kind = "system"
	KindProvider Kind = "provider"
)

// Entry is one row. PreviousValue and NewValue are the shape §10 asks for on a price change or
// a status override: what it was, what it is, and the reason the actor gave.
//
// Anything that could be a password, a token or a full card number must not be put here. The
// types are `any` so the caller cannot hide that decision in a string: pass the two fields that
// changed, not the struct you happened to have loaded.
type Entry struct {
	ActorID      *int64
	ActorKind    Kind
	Action       string
	ResourceType string
	ResourceID   *int64

	PreviousValue any
	NewValue      any
	Reason        string

	IP         string
	OccurredAt time.Time
}

// Writer appends one entry. The pgx adapter joins whatever transaction ctx already carries, so
// T-10 needs no coordinator in the calling code.
type Writer interface {
	Record(ctx context.Context, e Entry) error
}

var ErrInvalidEntry = errors.New("audit entry is not recordable")

// Validate catches the two mistakes that would silently produce an unreadable log: an action
// with no name, and a "user" actor with no user.
func (e Entry) Validate() error {
	if e.Action == "" {
		return fmt.Errorf("%w: action is required", ErrInvalidEntry)
	}
	switch e.ActorKind {
	case KindUser:
		if e.ActorID == nil {
			return fmt.Errorf("%w: a user actor needs an id, otherwise the entry cannot be read back", ErrInvalidEntry)
		}
	case KindSystem, KindProvider:
		if e.ActorID != nil {
			return fmt.Errorf("%w: a %s actor cannot carry a user id", ErrInvalidEntry, e.ActorKind)
		}
	default:
		return fmt.Errorf("%w: unknown actor kind %q", ErrInvalidEntry, e.ActorKind)
	}
	return nil
}

// SystemAction and ProviderAction build the entries where nobody is signed in — a background
// job, or Daraja. The audit log is the only witness for those, so they are constructors rather
// than an optional field the caller has to remember to set.
func SystemAction(action, resourceType string, resourceID *int64) Entry {
	return newEntry(KindSystem, action, resourceType, resourceID)
}

func ProviderAction(action, resourceType string, resourceID *int64) Entry {
	return newEntry(KindProvider, action, resourceType, resourceID)
}

// ByUser builds the entry for a signed-in actor's privileged action.
func ByUser(actorID int64, action, resourceType string, resourceID *int64) Entry {
	e := newEntry(KindUser, action, resourceType, resourceID)
	e.ActorID = &actorID
	return e
}

func newEntry(kind Kind, action, resourceType string, resourceID *int64) Entry {
	return Entry{ActorKind: kind, Action: action, ResourceType: resourceType, ResourceID: resourceID}
}
