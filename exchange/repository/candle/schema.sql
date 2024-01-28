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