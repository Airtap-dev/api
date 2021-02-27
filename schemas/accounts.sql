CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;

CREATE SEQUENCE IF NOT EXISTS public.user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.accounts (
    id bigint NOT NULL,
    license uuid NOT NULL,
    code character(12) NOT NULL,
    token character(32) NOT NULL,
    first_name character varying(30) NOT NULL,
    last_name character varying(30)
);

ALTER SEQUENCE public.user_id_seq OWNED BY public.accounts.id;
ALTER TABLE ONLY public.accounts ALTER COLUMN id SET DEFAULT nextval('public.user_id_seq'::regclass);
ALTER TABLE ONLY public.accounts ADD CONSTRAINT accounts_code_key UNIQUE (code);
ALTER TABLE ONLY public.accounts ADD CONSTRAINT accounts_pkey PRIMARY KEY (id);
ALTER TABLE ONLY public.license_keys ADD CONSTRAINT license_keys_pkey PRIMARY KEY (license);
