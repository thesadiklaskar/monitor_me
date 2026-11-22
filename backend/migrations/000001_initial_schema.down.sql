-- Reverse the initial control-plane schema in dependency order.

DROP TABLE IF EXISTS server_project_mappings;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS agent_enrollment_tokens;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS servers;
DROP TABLE IF EXISTS organization_memberships;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
