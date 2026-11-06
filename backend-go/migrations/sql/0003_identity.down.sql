-- 0003 down: identity and the audit log.
--
-- The reverse order of the up script, and the reason the seed data needs no explicit DELETE:
-- dropping the tables takes the seed with them, and re-running 0003 re-seeds from the matrix.
-- A down script that manually un-inserted seed rows would be a second copy of the catalogue to
-- keep in sync, which is the mistake this migration is replacing.
--
-- DESTRUCTIVE: drops every account, session and audit record in the schema. That is what
-- `make migrate-down` means on a local database and it is not reversible past this point — the
-- same trade-off 0001's down script states, with more reasons to care.

DROP TRIGGER IF EXISTS audit_logs_append_only ON administration.audit_logs;
DROP FUNCTION IF EXISTS identity.refuse_audit_mutation();
DROP TABLE IF EXISTS administration.audit_logs;

DROP TABLE IF EXISTS identity.invitations;
DROP TABLE IF EXISTS identity.sessions;
DROP TABLE IF EXISTS identity.user_permissions;
DROP TABLE IF EXISTS identity.role_permissions;
DROP TABLE IF EXISTS identity.permissions;
DROP TABLE IF EXISTS identity.user_roles;
DROP TABLE IF EXISTS identity.roles;

DROP TRIGGER IF EXISTS users_touch_updated_at ON identity.users;
DROP FUNCTION IF EXISTS identity.touch_updated_at();
DROP TABLE IF EXISTS identity.users;
