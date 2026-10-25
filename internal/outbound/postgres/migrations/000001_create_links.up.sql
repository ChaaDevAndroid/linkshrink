CREATE TABLE IF NOT EXISTS links (
  short_code TEXT PRIMARY KEY,
  long_url   TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);