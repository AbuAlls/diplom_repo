CREATE TABLE extracted_document_data (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL,
    recognized_text TEXT NOT NULL,
    structured_json JSONB NOT NULL,
    recognized_category_id BIGINT NULL,
    confidence_score NUMERIC(5, 2) NULL,
    processing_status VARCHAR(50) NOT NULL,
    processed_at TIMESTAMP NULL,
    model_version VARCHAR(100) NULL
);

CREATE TABLE audit_checks (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL,
    check_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    details_json JSONB NULL,
    processing_status VARCHAR(50) NOT NULL,
    checked_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE analytics_reports_items (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    report_type VARCHAR(100) NOT NULL,
    description TEXT NULL,
    source_scope VARCHAR(50) NOT NULL,
    source_id BIGINT NULL,
    period_type VARCHAR(50) NULL,
    date_from DATE NULL,
    date_to DATE NULL,
    payload_json JSONB NOT NULL,
    summary_text TEXT NOT NULL,
    plan_item_id BIGINT NULL
);

CREATE TABLE analytics_plans (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE analytics_plan_goals (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE analytics_relation_plan_to_goal (
    id BIGSERIAL PRIMARY KEY,
    analytics_plan_id BIGINT NOT NULL REFERENCES analytics_plans(id) ON DELETE CASCADE,
    analytics_plan_goal_id BIGINT NOT NULL REFERENCES analytics_plan_goals(id) ON DELETE CASCADE,
    UNIQUE (analytics_plan_id, analytics_plan_goal_id)
);

CREATE TABLE analytics_goal_report_relations (
    id BIGSERIAL PRIMARY KEY,
    analytics_plan_goal_id BIGINT NOT NULL REFERENCES analytics_plan_goals(id) ON DELETE CASCADE,
    analytics_report_item_id BIGINT NOT NULL REFERENCES analytics_reports_items(id) ON DELETE CASCADE,
    UNIQUE (analytics_plan_goal_id, analytics_report_item_id)
);

CREATE INDEX idx_extracted_document_data_document_id ON extracted_document_data(document_id);
CREATE INDEX idx_extracted_document_data_processing_status ON extracted_document_data(processing_status);

CREATE INDEX idx_audit_checks_document_id ON audit_checks(document_id);
CREATE INDEX idx_audit_checks_check_type ON audit_checks(check_type);
CREATE INDEX idx_audit_checks_status ON audit_checks(status);

CREATE INDEX idx_analytics_reports_items_source_scope_source_id
    ON analytics_reports_items(source_scope, source_id);

CREATE INDEX idx_analytics_relation_plan_to_goal_plan_id
    ON analytics_relation_plan_to_goal(analytics_plan_id);

CREATE INDEX idx_analytics_relation_plan_to_goal_goal_id
    ON analytics_relation_plan_to_goal(analytics_plan_goal_id);

CREATE INDEX idx_analytics_goal_report_relations_goal_id
    ON analytics_goal_report_relations(analytics_plan_goal_id);

CREATE INDEX idx_analytics_goal_report_relations_report_id
    ON analytics_goal_report_relations(analytics_report_item_id);