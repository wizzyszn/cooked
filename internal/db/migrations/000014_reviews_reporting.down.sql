DROP TRIGGER IF EXISTS trg_immutable_audit_logs ON audit_logs;
DROP FUNCTION IF EXISTS prevent_audit_log_mutation();
DROP TABLE IF EXISTS content_report_commands;
DROP TABLE IF EXISTS content_reports;
DROP TABLE IF EXISTS review_create_commands;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS recipe_version_review_aggregates;
