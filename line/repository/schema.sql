CREATE TABLE line_profile (
    id bigint(20) PRIMARY KEY,
    user_id bigint(20) NOT NULL,
    line_id text NOT NULL,
    display_name text NOT NULL,
    picture_url text NOT NULL,
    created_at timestamp NULL DEFAULT NULL,
    updated_at timestamp NULL DEFAULT NULL
);