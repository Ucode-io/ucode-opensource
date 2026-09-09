CREATE TYPE payment_type AS ENUM ('Bank', 'Payme', 'Stripe');

ALTER TABLE transaction 
ADD COLUMN payment_type payment_type NOT NULL DEFAULT 'Bank';
