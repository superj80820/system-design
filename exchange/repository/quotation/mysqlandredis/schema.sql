CREATE TABLE ticks (
  id BIGINT NOT NULL AUTO_INCREMENT,
  createdAt BIGINT NOT NULL,
  makerOrderId BIGINT NOT NULL,
  price DECIMAL(36, 18) NOT NULL,
  quantity DECIMAL(36, 18) NOT NULL,
  sequenceId BIGINT NOT NULL,
  takerDirection BIT NOT NULL,
  takerOrderId BIGINT NOT NULL,
  CONSTRAINT UNI_T_M UNIQUE (takerOrderId, makerOrderId),
  INDEX IDX_CAT (createdAt),
  PRIMARY KEY(id)
) CHARACTER SET utf8 COLLATE utf8_general_ci AUTO_INCREMENT = 1000;