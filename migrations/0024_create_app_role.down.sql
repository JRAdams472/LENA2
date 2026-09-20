DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'lena_app') THEN
        REASSIGN OWNED BY lena_app TO current_user;
        DROP OWNED BY lena_app;
        DROP ROLE lena_app;
    END IF;
END
$$;
