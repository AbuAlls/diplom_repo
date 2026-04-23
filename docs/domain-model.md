# Domain model

## Core
- users: system users
- groups: access groups / roles
- group_properties: property dictionary for groups
- user_group_relations: many-to-many users to groups
- folders: hierarchical document folders
- documents_categories: document categories
- documents: uploaded document metadata and extracted arrays
- plans: planning entities
- plan_goals: goals inside plan
- plan_items: measurable or actionable items
- plan_item_artifacts: documents/tasks/evidence linked to plan items

## Analysis
- extracted_document_data: OCR/LLM extraction results
- audit_checks: validation results for documents
- analytics_reports_item: generated reports
- analytics_plans: analytics views/projections of plans
- analytics_plan_goals: analytics views/projections of goals
- analytics_relation_plan_to_goal: relation table between analytics plans and goals