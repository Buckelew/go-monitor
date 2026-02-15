#!/bin/bash
set -e

if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

echo -e "Dropping databases"
dropdb --if-exists ${POSTGRES_DB_NAME}
dropdb --if-exists ${POSTGRES_TEST_DB_NAME}

echo -e "Creating databases"
createdb ${POSTGRES_DB_NAME}
createdb ${POSTGRES_TEST_DB_NAME}

echo -e "Running migrations"
make migrate-up

echo -e "Seeding data"
make seed
