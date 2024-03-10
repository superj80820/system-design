CREATE TABLE assets (
  id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  -- TODO: check
  asset_id BIGINT NOT NULL,
  available DECIMAL(36, 18) NOT NULL,
  frozen DECIMAL(36, 18) NOT NULL,
  sequence_id BIGINT NOT NULL,
  updated_at timestamp NULL DEFAULT NULL,
  created_at timestamp NULL DEFAULT NULL,
  CONSTRAINT unique_user_id_asset_id UNIQUE (user_id, asset_id),
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;