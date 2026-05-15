CREATE TABLE _scaffold (
    id          UUID PRIMARY KEY,
    org_id      UUID NOT NULL,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, external_id)
);

CREATE INDEX idx_scaffold_org_id ON _scaffold (org_id);
