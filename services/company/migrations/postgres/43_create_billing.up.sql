CREATE TYPE payment_status AS ENUM ('cancelled', 'accepted', 'pending');
CREATE TYPE transaction_type AS ENUM ('withdraw', 'topup');
CREATE TYPE creator_type AS ENUM ('system-user', 'user', 'server', 'transfer');
CREATE TYPE fare_item_type AS ENUM ('function', 'microfrontend', 'database', 'request_per_month', 'request_per_second', 'custom');

CREATE TABLE IF NOT EXISTS fare (
    id                  UUID PRIMARY KEY,
    name                VARCHAR(100) NOT NULL,
    currency            VARCHAR(15) NOT NULL DEFAULT 'uzs',
    price               DECIMAL(20,2) DEFAULT 0,
    trial_days          INTEGER NOT NULL DEFAULT 0,
    disactivate_day     INTEGER NOT NULL DEFAULT 0,
    description         VARCHAR(500) DEFAULT '',
    is_public           BOOLEAN DEFAULT FALSE,
    created_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at          INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS fare_item (
    id                  UUID PRIMARY KEY,
    name                VARCHAR(50) NOT NULL,
    parent_id           UUID REFERENCES fare_item(id) ON DELETE CASCADE, 
    info                VARCHAR(500) DEFAULT '',
    type                fare_item_type NOT NULL,
    created_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at          INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS fare_item_price (
    id                  UUID PRIMARY KEY,
    fare_id             UUID REFERENCES fare(id) ON DELETE CASCADE NOT NULL,
    fare_item_id        UUID REFERENCES fare_item(id) ON DELETE CASCADE NOT NULL,
    value               TEXT NOT NULL,
    price               DECIMAL(20,2) NOT NULL,
    over_limit_price    DECIMAL(20,2) NOT NULL,
    created_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at          INTEGER DEFAULT 0
);


CREATE TABLE IF NOT EXISTS transaction (
    id                      UUID PRIMARY KEY,
    project_id              UUID REFERENCES project(id) ON DELETE CASCADE,
    creator_id              UUID,
    comment                 VARCHAR(500) NOT NULL DEFAULT '',
    payment_status          payment_status NOT NULL,
    amount                  DECIMAL(20,2) NOT NULL,
    transaction_type        transaction_type NOT NULL,
    creator_type            creator_type NOT NULL,
    currency_id             UUID REFERENCES currency(id) ON DELETE CASCADE,
    acceptor_id             UUID,
    fare_id                 UUID REFERENCES fare(id) ON DELETE CASCADE,
    receipt_file            TEXT,
    created_at              TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    accepted_at             TIMESTAMP WITHOUT TIME ZONE
);

ALTER TABLE IF EXISTS project 
    ADD COLUMN IF NOT EXISTS balance        DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS credit_limit   DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS fare_id        UUID REFERENCES fare(id) ON DELETE SET NULL;

ALTER TABLE IF EXISTS transaction 
    ADD COLUMN IF NOT EXISTS order_id INTEGER;
