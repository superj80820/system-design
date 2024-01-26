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