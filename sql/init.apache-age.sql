CREATE EXTENSION IF NOT EXISTS age;

-- Ensure AGE catalog is visible by default for this database.
-- Update the database name if yours is different.
ALTER DATABASE "adk-agent" SET search_path = ag_catalog, "$user", public;
