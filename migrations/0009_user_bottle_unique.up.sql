ALTER TABLE wine.user_bottle
    ADD CONSTRAINT user_bottle_user_bottle_key UNIQUE (user_id, bottle_id);
