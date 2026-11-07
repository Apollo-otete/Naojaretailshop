-- 0004 down: the attribute conditions come off the catalogue.
--
-- Dropping the column takes its CHECK constraint with it, and re-applying 0004 re-derives the
-- declarations from the same literal key list, so there is no second copy of the `[own]` set to
-- keep in sync here. Non-destructive in the sense that matters: no table and no row this schema
-- owns disappears.

ALTER TABLE identity.permissions DROP COLUMN IF EXISTS conditions;
