ALTER TABLE ugen_template
    DROP COLUMN IF EXISTS source_repo_id,
    DROP COLUMN IF EXISTS source_function_id,
    DROP COLUMN IF EXISTS source_mcp_resource_env_id;
