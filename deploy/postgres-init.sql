-- Each service owns a database; projects get their own, created at runtime by
-- company-service on this same server (see NODE_POSTGRES_* in .env.example).
CREATE DATABASE auth_service;
CREATE DATABASE company_service;
CREATE DATABASE object_builder;
