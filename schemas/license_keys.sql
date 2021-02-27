CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

CREATE TABLE IF NOT EXISTS public.license_keys (
    license uuid DEFAULT public.uuid_generate_v4() NOT NULL UNIQUE,
    max_activations smallint DEFAULT 10 NOT NULL,
    revoked boolean DEFAULT false NOT NULL
);