#!/bin/bash
# Runs once, when the data volume is created.
#
# The migration suite is destructive by design — it drops all nine schemas to prove the
# rollback works — so it needs a database whose name says so. Sharing one database between
# `make run` and `make test-integration` means a test run wipes the catalogue you were looking
# at.
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	SELECT 'CREATE DATABASE naoja_test OWNER ${POSTGRES_USER}'
	WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'naoja_test')\gexec
EOSQL

echo "naoja_test is ready for make test-integration"
