CREATE TABLE ticks (
  id BIGINT NOT NULL AUTO_INCREMENT,
  created_at timestamp NULL DEFAULT NULL,
  maker_order_id BIGINT NOT NULL,
  price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  sequence_id BIGINT NOT NULL,
  taker_direction TINYINT NOT NULL,
  taker_order_id BIGINT NOT NULL,
  CONSTRAINT unique_taker_order_id_maker_order_id UNIQUE (taker_order_id, maker_order_id),
  INDEX index_create_at (created_at),
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;