CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS public.app_users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL,
    password_hash text NOT NULL,
    display_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    disabled_at timestamptz,
    CONSTRAINT app_users_email_normalized CHECK (email=lower(btrim(email))),
    CONSTRAINT app_users_display_name_length CHECK (
        char_length(display_name) BETWEEN 1 AND 100
    )
);
CREATE UNIQUE INDEX IF NOT EXISTS app_users_email_unique_idx
    ON public.app_users(email);

CREATE TABLE IF NOT EXISTS public.auth_sessions (
    token_hash bytea PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES public.app_users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz NOT NULL DEFAULT now(),
    user_agent text,
    CONSTRAINT auth_sessions_expiry CHECK (expires_at>created_at)
);
CREATE INDEX IF NOT EXISTS auth_sessions_user_idx
    ON public.auth_sessions(user_id,expires_at DESC);

CREATE TABLE IF NOT EXISTS public.user_preferences (
    user_id uuid PRIMARY KEY REFERENCES public.app_users(id) ON DELETE CASCADE,
    preferences jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_preferences_object CHECK (jsonb_typeof(preferences)='object')
);

ALTER ROLE real_estate_api RESET default_transaction_read_only;
REVOKE INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER
    ON ALL TABLES IN SCHEMA public,analytics FROM real_estate_api;
GRANT SELECT ON ALL TABLES IN SCHEMA public,analytics TO real_estate_api;
GRANT INSERT,UPDATE ON public.app_users TO real_estate_api;
GRANT INSERT,SELECT,UPDATE,DELETE ON public.auth_sessions TO real_estate_api;
GRANT INSERT,SELECT,UPDATE,DELETE ON public.user_preferences TO real_estate_api;
GRANT UPDATE(review_status,review_notes,reviewed_by,reviewed_at)
    ON analytics.anomaly_signals TO real_estate_api;

COMMENT ON TABLE public.app_users IS
    'Application identities; passwords are stored only as adaptive hashes.';
COMMENT ON TABLE public.auth_sessions IS
    'Opaque bearer sessions stored as SHA-256 token hashes.';
COMMENT ON TABLE public.user_preferences IS
    'Small version-flexible JSON object containing saved UI and analysis preferences.';
