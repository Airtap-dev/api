CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

CREATE SEQUENCE IF NOT EXISTS public.license_keys_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.license_keys (
    id bigint NOT NULL UNIQUE,
    license uuid DEFAULT public.uuid_generate_v4() NOT NULL UNIQUE,
    max_activations smallint DEFAULT 10 NOT NULL,
    revoked boolean DEFAULT false NOT NULL,

    PRIMARY KEY (id)
);

ALTER SEQUENCE public.license_keys_seq OWNED BY public.license_keys.id;
ALTER TABLE ONLY public.license_keys ALTER COLUMN id SET DEFAULT nextval('public.license_keys_seq'::regclass);