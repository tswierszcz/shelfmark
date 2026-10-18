ALTER TABLE libraries ADD COLUMN is_direct boolean NOT NULL DEFAULT true;

CREATE TABLE strategic_marks (
    library_id    uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    category_id   uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    active        boolean NOT NULL DEFAULT true,
    justification text NOT NULL,
    marked_by     text NOT NULL,
    marked_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (library_id, category_id)
);

CREATE INDEX strategic_marks_active_idx ON strategic_marks (category_id) WHERE active;
