-- Restore the FK constraints to mcp_project. Only safe if all existing rows
-- in agents and agent_runs still reference valid mcp_project rows.

ALTER TABLE agents
    ADD CONSTRAINT agents_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES mcp_project (id) ON DELETE CASCADE;

ALTER TABLE agent_runs
    ADD CONSTRAINT agent_runs_project_id_fkey
        FOREIGN KEY (project_id) REFERENCES mcp_project (id) ON DELETE CASCADE;