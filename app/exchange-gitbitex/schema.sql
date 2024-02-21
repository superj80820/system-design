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
CREATE TABLE sec_bars (
  start_time BIGINT NOT NULL,
  close_price DECIMAL(36, 18) NOT NULL,
  high_price DECIMAL(36, 18) NOT NULL,
  low_price DECIMAL(36, 18) NOT NULL,
  open_price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  PRIMARY KEY(start_time)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;

CREATE TABLE min_bars (
  start_time BIGINT NOT NULL,
  close_price DECIMAL(36, 18) NOT NULL,
  high_price DECIMAL(36, 18) NOT NULL,
  low_price DECIMAL(36, 18) NOT NULL,
  open_price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  PRIMARY KEY(start_time)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;

CREATE TABLE hour_bars (
  start_time BIGINT NOT NULL,
  close_price DECIMAL(36, 18) NOT NULL,
  high_price DECIMAL(36, 18) NOT NULL,
  low_price DECIMAL(36, 18) NOT NULL,
  open_price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  PRIMARY KEY(start_time)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;

CREATE TABLE day_bars (
  start_time BIGINT NOT NULL,
  close_price DECIMAL(36, 18) NOT NULL,
  high_price DECIMAL(36, 18) NOT NULL,
  low_price DECIMAL(36, 18) NOT NULL,
  open_price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  PRIMARY KEY(start_time)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;
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
CREATE TABLE `account` (
  `id` bigint(20) NOT NULL,
  `email` varchar(255) NOT NULL,
  `password` varchar(255) NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `unique_email` (`email`),
  INDEX `index_email` (`email`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8;

CREATE TABLE `account_token` (
  `id` bigint(20) NOT NULL,
  `token` varchar(255) NOT NULL,
  `status` tinyint(1) NOT NULL DEFAULT '0',
  `type` int(11) NOT NULL DEFAULT '0',
  -- user_id to account_id
  `user_id` bigint(20) NOT NULL,
  `expire_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  -- index_user_id to index_account_id
  INDEX `index_user_id` (`user_id`),
  UNIQUE KEY `unique_token` (`token`),
  -- user_id to account_id
  FOREIGN KEY (`user_id`) REFERENCES account(`id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8;