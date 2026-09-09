ALTER TABLE company
ADD COLUMN is_deleted boolean DEFAULT false,
ADD COLUMN deleted_at timestamp;

ALTER TABLE project
ADD COLUMN is_deleted boolean DEFAULT false,
ADD COLUMN deleted_at timestamp;