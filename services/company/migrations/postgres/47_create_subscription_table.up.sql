CREATE TABLE IF NOT EXISTS discount (
    id                  UUID PRIMARY KEY,
    months              INTEGER NOT NULL DEFAULT 1,
    value               DECIMAL(5,2) NOT NULL DEFAULT 0,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS subscription (
    id              UUID PRIMARY KEY,
    project_id      UUID REFERENCES project(id) ON DELETE CASCADE NOT NULL,
    fare_id         UUID REFERENCES fare(id) ON DELETE CASCADE NOT NULL,
    discount_id     UUID REFERENCES discount(id) ON DELETE CASCADE,
    status          VARCHAR(20) NOT NULL DEFAULT 'active' 
                    CHECK (status IN ('active', 'canceled', 'paused', 'expired')),
    start_date      DATE NOT NULL DEFAULT CURRENT_DATE,
    end_date        DATE NOT NULL, 
    renewal_date    DATE NOT NULL,
    auto_renew      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE IF EXISTS "transaction"
    ADD COLUMN IF NOT EXISTS subscription_id UUID REFERENCES subscription(id) ON DELETE CASCADE;

ALTER TABLE IF EXISTS fare
    DROP COLUMN IF EXISTS currency,
    ADD COLUMN IF NOT EXISTS currency_id UUID REFERENCES currency(id) ON DELETE SET NULL DEFAULT '88c816a3-24e8-4994-ab70-9bc826bb9dc3';

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'project_status'
    ) THEN
        ALTER TYPE project_status ADD VALUE IF NOT EXISTS 'insufficient_funds';
    END IF;
END;
$$;

DO
$$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_type
        WHERE typname = 'transaction_type'
    ) THEN
        ALTER TYPE transaction_type ADD VALUE IF NOT EXISTS 'subscription';
    END IF;
END;
$$;

-- The hosted product's price list used to be seeded here. The open-source
-- build has no billing: the service code that read these tables was removed,
-- so the tables ship empty and the rows would only be commercial data with
-- nothing to read them. The schema itself stays, so a fork that wants billing
-- has somewhere to put it.
