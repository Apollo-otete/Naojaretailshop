-- 0001 down: undo the context schemas.
--
-- CASCADE is deliberate. A migration's down script has to work in the state the up script
-- produced plus whatever the application put in it — dropping each schema only if it is empty
-- would make the rollback fail on exactly the databases where a rollback is actually needed.
-- The cost is stated plainly: everything inside these schemas is destroyed, which is why this
-- pair is tested against a scratch database, never against a production one.

DROP SCHEMA IF EXISTS administration CASCADE;
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP SCHEMA IF EXISTS engagement CASCADE;
DROP SCHEMA IF EXISTS fulfilment CASCADE;
DROP SCHEMA IF EXISTS inventory CASCADE;
DROP SCHEMA IF EXISTS payments CASCADE;
DROP SCHEMA IF EXISTS commerce CASCADE;
DROP SCHEMA IF EXISTS catalogue CASCADE;
DROP SCHEMA IF EXISTS identity CASCADE;

DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS citext;
