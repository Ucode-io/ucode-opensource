    ALTER TABLE "session" ADD CONSTRAINT "session_client_platform_id_fkey" FOREIGN KEY ("client_platform_id") REFERENCES "client_platform"("id");
    ALTER TABLE "session" ADD CONSTRAINT "session_client_type_id_fkey" FOREIGN KEY ("client_type_id") REFERENCES "client_type"("id");
    ALTER TABLE "session" ADD CONSTRAINT "session_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "user"("id");
    ALTER TABLE "session" ADD CONSTRAINT "session_role_id_fkey" FOREIGN KEY ("role_id") REFERENCES "role"("id");
    ALTER TABLE "scope" ADD CONSTRAINT "scope_client_platform_id_fkey" FOREIGN KEY ("client_platform_id") REFERENCES "client_platform"("id");
