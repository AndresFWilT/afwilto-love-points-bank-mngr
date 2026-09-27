CREATE TABLE members (
    id           uuid        PRIMARY KEY,
    display_name text        NOT NULL,
    color        text        NOT NULL DEFAULT '#1B2A4A',
    created_at   timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT members_display_name_not_blank
        CHECK (btrim(display_name) <> '')
);
