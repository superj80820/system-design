CREATE TABLE orders (
  id BIGINT NOT NULL,
  created_at timestamp NULL DEFAULT NULL,
  direction VARCHAR(32) NOT NULL,
  price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  sequence_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL,
  unfilled_quantity DECIMAL(36, 18) NOT NULL,
  updated_at timestamp NULL DEFAULT NULL,
  user_id BIGINT NOT NULL,
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;

CREATE TABLE events (
  reference_id BIGINT(20) NOT NULL,
  sequence_id BIGINT NOT NULL,
  previous_id BIGINT NOT NULL,
  created_at timestamp NULL DEFAULT NULL,
  data VARCHAR(10000) NOT NULL,
  CONSTRAINT unique_previous_id UNIQUE (previous_id),
  CONSTRAINT unique_sequence_id UNIQUE (sequence_id),
  PRIMARY KEY(reference_id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;