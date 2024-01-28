-- TODO: check
CREATE TABLE match_details (
  id BIGINT NOT NULL AUTO_INCREMENT,
  counter_order_id BIGINT NOT NULL,
  counter_user_id BIGINT NOT NULL,
  created_at timestamp NULL DEFAULT NULL,
  direction VARCHAR(32) NOT NULL,
  order_id BIGINT NOT NULL,
  price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  sequence_id BIGINT NOT NULL,
  type VARCHAR(32) NOT NULL,
  user_id BIGINT NOT NULL,
  CONSTRAINT unique_order_id_counter_order_id UNIQUE (order_id, counter_order_id),
  INDEX unique_order_id_created_at (order_id, created_at),
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;