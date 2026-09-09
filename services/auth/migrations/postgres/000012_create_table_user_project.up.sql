create table if not exists user_project (
    user_id uuid not null references "user"("id"),
    company_id uuid not null,
    project_id uuid not null,
    UNIQUE ("user_id", "company_id", "project_id")
)