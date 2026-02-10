alter table config_keys
    add column if not exists value_json jsonb null;