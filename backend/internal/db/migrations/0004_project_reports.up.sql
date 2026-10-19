CREATE TABLE projects (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    created_by text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX projects_name_lower_idx ON projects (lower(name));

CREATE TABLE project_libraries (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    library_id uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    is_direct  boolean NOT NULL,
    PRIMARY KEY (project_id, library_id)
);
