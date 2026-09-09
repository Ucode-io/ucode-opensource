
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS language_id UUID REFERENCES "language"("id");
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS timezone_id UUID REFERENCES "timezone"("id");