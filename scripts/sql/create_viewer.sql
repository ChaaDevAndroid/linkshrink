DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'viewer') THEN
    CREATE ROLE viewer LOGIN;
  END IF;
END
$$;

ALTER ROLE viewer SET default_transaction_read_only = on;
ALTER ROLE viewer SET statement_timeout = '30s';
GRANT CONNECT ON DATABASE linkshrink TO viewer;
GRANT USAGE ON SCHEMA public TO viewer;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO viewer;
ALTER DEFAULT PRIVILEGES FOR ROLE linkshrink IN SCHEMA public GRANT SELECT ON TABLES TO viewer;