CREATE TABLE IF NOT EXISTS ugen_currency_rate_cache (
    currency_code   VARCHAR(16) NOT NULL,
    rate_date       DATE NOT NULL,
    rate            DECIMAL(20,4) NOT NULL CHECK (rate > 0),
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (currency_code, rate_date)
);
