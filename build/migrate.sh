#!/usr/bin/env bash
set -euo pipefail
set -a
source .env
set +a

COMPOSE_FILE="compose.yaml"
MYSQL_SERVICE="mysql"
DB="mysql://${MYSQL_USER}:${MYSQL_PASSWORD}@127.0.0.1:3306/${MYSQL_DATABASE}?parseTime=true"

echo "Stopping all running Docker containers..."
RUNNING_CONTAINERS="$(docker ps -q)"
if [[ -n "${RUNNING_CONTAINERS}" ]]; then
  docker stop ${RUNNING_CONTAINERS}
else
  echo "No running containers."
fi

echo "Starting compose service '${MYSQL_SERVICE}' (detached)..."
docker compose -f "${COMPOSE_FILE}" up -d "${MYSQL_SERVICE}"

echo "Applying schema migrations with dbmate..."
dbmate --wait -u "${DB}" up

echo "Dumping schema with dbmate..."
dbmate --wait -u "${DB}" dump

echo "Stopping compose service '${MYSQL_SERVICE}'..."
docker compose -f "${COMPOSE_FILE}" down

echo "Done."
