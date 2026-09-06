CREATE TABLE IF NOT EXISTS metrics (
    id TEXT NOT NULL,
    mtype TEXT NOT NULL,
    value DOUBLE PRECISION,
    delta BIGINT,
    PRIMARY KEY (id, mtype)
);
