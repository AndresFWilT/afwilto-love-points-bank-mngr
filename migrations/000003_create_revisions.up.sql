CREATE TABLE point_entry_revisions (
    id               uuid        PRIMARY KEY,
    entry_id         uuid        NOT NULL REFERENCES point_entries(id) ON DELETE CASCADE,
    edited_by        uuid        NOT NULL REFERENCES members(id) ON DELETE RESTRICT,
    prev_amount      smallint    NOT NULL,
    prev_reason      text        NOT NULL,
    prev_occurred_at timestamptz NOT NULL,
    edited_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX point_entry_revisions_entry_idx
    ON point_entry_revisions (entry_id, edited_at DESC);
