-- agents.project_id and agent_runs.project_id previously referenced mcp_project(id).
-- Agents are keyed by resource_environment, not by the generated frontend project,
-- so the FK is wrong and breaks inserts that pass a resource_env_id as project_id.
-- Drop the FK constraints; the UUID columns remain NOT NULL and unconstrained.

ALTER TABLE agents
    DROP CONSTRAINT IF EXISTS agents_project_id_fkey;

ALTER TABLE agent_runs
    DROP CONSTRAINT IF EXISTS agent_runs_project_id_fkey;
