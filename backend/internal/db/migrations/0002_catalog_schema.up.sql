CREATE TABLE categories (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text UNIQUE NOT NULL
);

INSERT INTO categories (name) VALUES
    ('logging'),
    ('http-client'),
    ('serialization'),
    ('orm-persistence'),
    ('dependency-injection'),
    ('testing'),
    ('build-tooling'),
    ('security-crypto'),
    ('messaging'),
    ('metrics-observability'),
    ('web-framework'),
    ('utility'),
    ('json-processing'),
    ('validation');

CREATE TABLE libraries (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    purl             text UNIQUE NOT NULL,
    name             text NOT NULL,
    ecosystem        text NOT NULL DEFAULT 'maven',
    github_repo      text,
    status           text NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending', 'processing', 'categorized', 'needs_manual', 'failed')),
    upload_batch_id  uuid NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX libraries_upload_batch_id_idx ON libraries (upload_batch_id);
CREATE INDEX libraries_status_idx ON libraries (status);

CREATE TABLE library_categories (
    library_id  uuid NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    category_id uuid NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    source      text NOT NULL CHECK (source IN ('github', 'llm', 'manual')),
    PRIMARY KEY (library_id, category_id)
);
