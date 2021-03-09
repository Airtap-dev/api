ALTER TABLE IF EXISTS public.license_keys DROP COLUMN created_at;
ALTER TABLE IF EXISTS public.license_keys DROP COLUMN last_updated_at;
ALTER TABLE IF EXISTS public.accounts DROP COLUMN created_at;
ALTER TABLE IF EXISTS public.accounts DROP COLUMN last_updated_at;

DROP FUNCTION IF EXISTS set_last_updated_at_column CASCADE;
