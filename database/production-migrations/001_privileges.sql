-- Passwords and LOGIN are reconciled by CNPG managed roles, never this file.
REVOKE CREATE ON DATABASE real_estate FROM PUBLIC, real_estate_api, real_estate_pipeline;
REVOKE CREATE ON SCHEMA public FROM PUBLIC, real_estate_api, real_estate_pipeline;
REVOKE CREATE ON SCHEMA analytics FROM PUBLIC, real_estate_api, real_estate_pipeline;
GRANT CONNECT, TEMPORARY ON DATABASE real_estate TO real_estate_pipeline;
GRANT USAGE ON SCHEMA public, analytics TO real_estate_pipeline;
GRANT SELECT ON ALL TABLES IN SCHEMA public, analytics TO real_estate_pipeline;
GRANT INSERT, UPDATE, DELETE, TRUNCATE ON ALL TABLES IN SCHEMA public, analytics TO real_estate_pipeline;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public, analytics TO real_estate_pipeline;
REVOKE ALL ON public.production_schema_migrations FROM real_estate_pipeline, real_estate_api;
REVOKE ALL ON public.app_users, public.auth_sessions, public.user_preferences FROM real_estate_pipeline;
REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA analytics FROM PUBLIC, real_estate_api;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA analytics TO real_estate_pipeline;
-- API needs read-only classification helpers, not refresh/publish/admin functions.
DO $$
DECLARE f record;
BEGIN
  FOR f IN SELECT p.oid::regprocedure AS signature
           FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
           WHERE n.nspname='analytics'
             AND p.proname NOT LIKE 'refresh_%'
             AND p.proname NOT LIKE 'publish_%'
             AND p.provolatile IN ('s','i')
  LOOP
    EXECUTE format('GRANT EXECUTE ON FUNCTION %s TO real_estate_api', f.signature);
  END LOOP;
  -- PostgreSQL 16 ANALYZE requires table ownership. Only the fixed, reviewed
  -- refresh entry points execute with their migration owner's privileges.
  -- Untrusted schemas (including implicit pg_temp) cannot shadow their sources.
  FOR f IN SELECT p.oid::regprocedure AS signature
           FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
           WHERE n.nspname='analytics' AND p.proname LIKE 'refresh_%'
  LOOP
    EXECUTE format('ALTER FUNCTION %s SECURITY DEFINER', f.signature);
    EXECUTE format(
      'ALTER FUNCTION %s SET search_path = pg_catalog, public, analytics, pg_temp',
      f.signature);
  END LOOP;
END $$;
ALTER DEFAULT PRIVILEGES IN SCHEMA analytics REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC;
-- Future migrations must explicitly grant newly created tables/functions.
