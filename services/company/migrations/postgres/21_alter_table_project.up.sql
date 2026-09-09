ALTER TABLE "project" ADD COLUMN "language_id" UUID REFERENCES "language"("id");
ALTER TABLE "project" ADD COLUMN "currency_id" UUID REFERENCES "currency"("id");
ALTER TABLE "project" ADD COLUMN "timezone_id" UUID REFERENCES "timezone"("id");