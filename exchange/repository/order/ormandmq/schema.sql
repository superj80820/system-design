CREATE TABLE orders (
  id BIGINT NOT NULL,
  created_at timestamp NULL DEFAULT NULL,
  direction VARCHAR(32) NOT NULL,
  price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  sequence_id BIGINT NOT NULL,
  status tinyint(1) NOT NULL DEFAULT '0',
  unfilled_quantity DECIMAL(36, 18) NOT NULL,
  updated_at timestamp NULL DEFAULT NULL,
  user_id BIGINT NOT NULL,
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;