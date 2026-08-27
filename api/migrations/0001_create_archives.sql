CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS archives (
  filename text PRIMARY KEY,
  original_name text,
  title text,
  version text,
  summary text,
  uploaded_at timestamptz,
  url text
);
