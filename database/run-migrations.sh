#!/usr/bin/env bash
set -euo pipefail

database_url="postgresql://real_estate:real_estate@db:5432/real_estate"

until pg_isready -h db -U real_estate -d real_estate >/dev/null 2>&1; do
  sleep 1
done

psql -v ON_ERROR_STOP=1 "$database_url" <<'SQL'
CREATE TABLE IF NOT EXISTS public.schema_migrations (
    version integer PRIMARY KEY,
    filename text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE public.schema_migrations IS
    'Incremental database migrations applied after the supported schema baseline.';
SQL

for migration in /migrations/[0-9][0-9][0-9]_*.sql; do
  filename="${migration##*/}"
  version_text="${filename%%_*}"
  version=$((10#$version_text))

  # Migrations 001-025 form the supported baseline and are installed by the
  # database image on a new volume. Runtime tracking starts with migration 026.
  if (( version < 26 )); then
    continue
  fi

  applied="$(psql -At -v ON_ERROR_STOP=1 "$database_url" \
    -c "SELECT EXISTS(SELECT 1 FROM public.schema_migrations WHERE version=$version)")"
  if [[ "$applied" == "t" ]]; then
    echo "Already applied $filename"
    continue
  fi

  echo "Applying $filename"
  psql -v ON_ERROR_STOP=1 "$database_url" -f "$migration"
  psql -v ON_ERROR_STOP=1 "$database_url" \
    -c "INSERT INTO public.schema_migrations(version,filename) VALUES($version,'$filename')"
done
