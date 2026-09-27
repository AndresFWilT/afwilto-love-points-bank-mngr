CREATE TYPE redemption_status AS ENUM ('active', 'voided');

CREATE TABLE redemptions (
    id                 uuid              PRIMARY KEY,
    member_id          uuid              NOT NULL REFERENCES members(id) ON DELETE RESTRICT,
    catalog_item_id    uuid              NOT NULL REFERENCES catalog_items(id) ON DELETE RESTRICT,
    cost_at_redemption smallint          NOT NULL,
    note               text              NOT NULL,
    status             redemption_status NOT NULL DEFAULT 'active',
    redeemed_at        timestamptz       NOT NULL DEFAULT now(),
    updated_at         timestamptz       NOT NULL DEFAULT now(),

    -- RD-CAN-1: se gasta una cantidad positiva
    CONSTRAINT redemptions_amount_positive CHECK (cost_at_redemption > 0),
    -- RD-CAN-9: el porqué del canje es obligatorio, como en un abono
    CONSTRAINT redemptions_note_min_length CHECK (char_length(btrim(note)) >= 10)
);

CREATE INDEX redemptions_member_active_idx
    ON redemptions (member_id) WHERE status = 'active';
CREATE INDEX redemptions_ledger_idx
    ON redemptions (redeemed_at DESC, id DESC);
