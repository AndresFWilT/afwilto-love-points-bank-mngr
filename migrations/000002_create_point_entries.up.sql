CREATE TYPE entry_status AS ENUM ('active', 'voided');

CREATE TABLE point_entries (
    id          uuid         PRIMARY KEY,
    awarded_to  uuid         NOT NULL REFERENCES members(id) ON DELETE RESTRICT,
    awarded_by  uuid         NOT NULL REFERENCES members(id) ON DELETE RESTRICT,
    amount      smallint     NOT NULL,
    reason      text         NOT NULL,
    occurred_at timestamptz  NOT NULL,
    status      entry_status NOT NULL DEFAULT 'active',
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),

    -- RD-1: nunca cero, nunca negativo, nunca inflación
    CONSTRAINT point_entries_amount_range
        CHECK (amount BETWEEN 1 AND 5),

    -- RD-2: la justificación es la regla del juego
    CONSTRAINT point_entries_reason_min_length
        CHECK (char_length(btrim(reason)) >= 10),

    -- RD-3: nadie se abona a sí mismo
    CONSTRAINT point_entries_no_self_award
        CHECK (awarded_by <> awarded_to)
);

CREATE INDEX point_entries_ledger_idx
    ON point_entries (occurred_at DESC, id DESC);

CREATE INDEX point_entries_balance_idx
    ON point_entries (awarded_to)
    WHERE status = 'active';
