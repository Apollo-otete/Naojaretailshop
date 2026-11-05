-- 0002 down.
--
-- Reversible here means "the schema returns to the state before this migration". Seed rows
-- that an operator edited after applying are destroyed by the rollback — that is the correct
-- trade for a migration pair a reviewer can reason about, and it is why §12 step 3 requires a
-- verified backup before any migration runs against a real database.

DROP TABLE IF EXISTS administration.system_settings;
