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
    code character(12) NOT NULL UNIQUE,
    token character(128) NOT NULL,
    first_name character varying(30) NOT NULL,
    last_name character varying(30),

    PRIMARY KEY (id),
    CONSTRAINT fk_license
        FOREIGN KEY(license)
            REFERENCES license_keys(license)
);

ALTER SEQUENCE public.user_id_seq OWNED BY public.accounts.id;
ALTER TABLE ONLY public.accounts ALTER COLUMN id SET DEFAULT nextval('public.user_id_seq'::regclass);
