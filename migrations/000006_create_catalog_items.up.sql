CREATE TYPE catalog_tier AS ENUM ('baratito', 'medio', 'alto');

CREATE TABLE catalog_items (
    id          uuid         PRIMARY KEY,
    name        text         NOT NULL,
    description text         NOT NULL DEFAULT '',
    cost        smallint     NOT NULL,
    is_active   boolean      NOT NULL DEFAULT true,
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),

    -- RD-CAT-1: el costo es positivo
    CONSTRAINT catalog_items_cost_positive CHECK (cost > 0),
    -- RD-CAT-2: el nombre no está en blanco (el mínimo útil lo valida el dominio)
    CONSTRAINT catalog_items_name_not_blank CHECK (btrim(name) <> ''),

    -- RD-CAT-3: el nivel es columna GENERADA a partir del costo. La base es la última
    -- línea de defensa; el dominio tiene la misma función TierFor(cost).
    tier catalog_tier GENERATED ALWAYS AS (
        CASE
            WHEN cost <= 5  THEN 'baratito'::catalog_tier
            WHEN cost <= 50 THEN 'medio'::catalog_tier
            ELSE                 'alto'::catalog_tier
        END
    ) STORED
);

CREATE INDEX catalog_items_active_idx ON catalog_items (is_active) WHERE is_active;
