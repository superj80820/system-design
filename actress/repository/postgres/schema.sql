-- actresses table Definition ----------------------------------------------

CREATE TABLE actresses (
    id bigserial PRIMARY KEY,
    name character varying(255),
    romanization character varying(255),
    detail character varying(255),
    preview character varying(255),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

-- actresses indices -------------------------------------------------------

CREATE UNIQUE INDEX idx_16422_primary ON actresses(id int8_ops);
CREATE UNIQUE INDEX idx_16422_name ON actresses(name text_ops);


-- actress_faces table Definition ----------------------------------------------

CREATE TABLE actress_faces (
    id bigserial PRIMARY KEY,
    token character varying(255),
    preview character varying(255),
    actress_id bigint REFERENCES actresses(id) ON DELETE CASCADE ON UPDATE RESTRICT,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    token_set character varying(255),
    status smallint
);

-- actress_faces indices -------------------------------------------------------

CREATE UNIQUE INDEX idx_16411_primary ON actress_faces(id int8_ops);
CREATE INDEX idx_16411_actress_id ON actress_faces(actress_id int8_ops);
CREATE INDEX idx_16411_token ON actress_faces(token text_ops);


-- account_favorite_actresses table Definition ----------------------------------------------

CREATE TABLE account_favorite_actresses (
    id text PRIMARY KEY,
    actress_id bigint REFERENCES actresses(id) ON DELETE CASCADE ON UPDATE RESTRICT,
    account_id bigint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone
);

-- account_favorite_actresses indices -------------------------------------------------------

CREATE UNIQUE INDEX facefavorites_pkey ON account_favorite_actresses(id text_ops);
CREATE UNIQUE INDEX idx_account_id_actress_id ON account_favorite_actresses(account_id int8_ops,actress_id int8_ops);
