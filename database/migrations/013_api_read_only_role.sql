DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='real_estate_api') THEN
        CREATE ROLE real_estate_api LOGIN PASSWORD 'real_estate_api';
    END IF;
END
$$;

ALTER ROLE real_estate_api SET default_transaction_read_only=on;
GRANT CONNECT ON DATABASE real_estate TO real_estate_api;
GRANT USAGE ON SCHEMA public,analytics TO real_estate_api;
GRANT SELECT ON ALL TABLES IN SCHEMA public,analytics TO real_estate_api;
ALTER DEFAULT PRIVILEGES FOR ROLE real_estate IN SCHEMA public
    GRANT SELECT ON TABLES TO real_estate_api;
ALTER DEFAULT PRIVILEGES FOR ROLE real_estate IN SCHEMA analytics
    GRANT SELECT ON TABLES TO real_estate_api;

COMMENT ON ROLE real_estate_api IS
    'Read-only login used by the Go analytics API; development password only.';
