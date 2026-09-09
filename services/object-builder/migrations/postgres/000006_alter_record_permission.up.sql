ALTER TABLE "record_permission"
    ADD COLUMN "add_filter" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "field_filter" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "fix_column" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "columns" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "group" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "excel_menu" VARCHAR(255) DEFAULT 'Yes',
    ADD COLUMN "tab_group" VARCHAR(255) DEFAULT 'Yes';
