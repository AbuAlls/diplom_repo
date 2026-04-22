CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    full_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE groups (
    id BIGSERIAL PRIMARY KEY,
    description VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE properties (
    id BIGSERIAL PRIMARY KEY,
    description VARCHAR(255) NOT NULL
);

CREATE TABLE group_properties (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    property_id BIGINT NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    UNIQUE (group_id, property_id)
);

CREATE TABLE user_group_relations (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (group_id, user_id)
);

CREATE TABLE plans (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE plan_goals (
    id BIGSERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE plan_items (
    id BIGSERIAL PRIMARY KEY,
    goal_id BIGINT NOT NULL REFERENCES plan_goals(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NULL,
    item_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    target_value NUMERIC(10, 2) NULL,
    current_value NUMERIC(10, 2) NULL,
    unit VARCHAR(50) NULL,
    progress_percent NUMERIC(5, 2) NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    historic_values JSONB NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE folders (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    parent_folder_id BIGINT NULL REFERENCES folders(id) ON DELETE SET NULL,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    plan_id BIGINT NULL REFERENCES plans(id) ON DELETE SET NULL,
    plan_goal_id BIGINT NULL REFERENCES plan_goals(id) ON DELETE SET NULL,
    plan_item_id BIGINT NULL REFERENCES plan_items(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CHECK (
        (CASE WHEN plan_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN plan_goal_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN plan_item_id IS NOT NULL THEN 1 ELSE 0 END)
        <= 1
    )
);

CREATE TABLE group_folder_relations (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    folder_id BIGINT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
    UNIQUE (group_id, folder_id)
);

CREATE TABLE group_plan_relations (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    plan_id BIGINT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    UNIQUE (group_id, plan_id)
);

CREATE TABLE document_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    code VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NULL
);

CREATE TABLE documents (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    folder_id BIGINT NOT NULL REFERENCES folders(id) ON DELETE RESTRICT,
    category_id BIGINT NULL REFERENCES document_categories(id) ON DELETE SET NULL,
    uploaded_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(255) NOT NULL,
    document_date DATE NULL,
    external_number VARCHAR(100) NULL,
    organization_name VARCHAR(255) NULL,
    inn VARCHAR(20) NULL,
    description TEXT NULL,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NULL,
    deadlines DATE[] NULL,
    personal_data TEXT[] NULL,
    organization_data TEXT[] NULL,
    prices TEXT[] NULL,
    quantities INT[] NULL,
    product_names TEXT[] NULL,
    contract_numbers TEXT[] NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE plan_item_artifacts (
    id BIGSERIAL PRIMARY KEY,
    plan_item_id BIGINT NOT NULL REFERENCES plan_items(id) ON DELETE CASCADE,
    artifact_type VARCHAR(50) NOT NULL,
    document_id BIGINT NULL REFERENCES documents(id) ON DELETE SET NULL,
    task_title VARCHAR(255) NULL,
    task_description TEXT NULL,
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMP NULL,
    evidence_document_id BIGINT NULL REFERENCES documents(id) ON DELETE SET NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_folders_parent_folder_id ON folders(parent_folder_id);
CREATE INDEX idx_folders_created_by ON folders(created_by);

CREATE INDEX idx_documents_folder_id ON documents(folder_id);
CREATE INDEX idx_documents_category_id ON documents(category_id);
CREATE INDEX idx_documents_uploaded_by ON documents(uploaded_by);
CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_document_date ON documents(document_date);

CREATE INDEX idx_plan_goals_plan_id ON plan_goals(plan_id);
CREATE INDEX idx_plan_items_goal_id ON plan_items(goal_id);
CREATE INDEX idx_plan_item_artifacts_plan_item_id ON plan_item_artifacts(plan_item_id);

CREATE INDEX idx_user_group_relations_user_id ON user_group_relations(user_id);
CREATE INDEX idx_user_group_relations_group_id ON user_group_relations(group_id);
CREATE INDEX idx_group_folder_relations_group_id ON group_folder_relations(group_id);
CREATE INDEX idx_group_folder_relations_folder_id ON group_folder_relations(folder_id);
CREATE INDEX idx_group_plan_relations_group_id ON group_plan_relations(group_id);
CREATE INDEX idx_group_plan_relations_plan_id ON group_plan_relations(plan_id);