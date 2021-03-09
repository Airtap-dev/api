ALTER TABLE IF EXISTS public.license_keys ADD created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE IF EXISTS public.license_keys ADD last_updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE IF EXISTS public.accounts ADD created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE IF EXISTS public.accounts ADD last_updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

CREATE OR REPLACE FUNCTION set_last_updated_at_column()   
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_updated_at = now();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_time BEFORE UPDATE ON public.accounts FOR EACH ROW EXECUTE PROCEDURE set_last_updated_at_column();
CREATE TRIGGER update_time BEFORE UPDATE ON public.license_keys FOR EACH ROW EXECUTE PROCEDURE set_last_updated_at_column();
