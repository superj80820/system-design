CREATE TABLE `account` (
  `id` bigint(20) NOT NULL PRIMARY KEY,
  `email` varchar(255) NOT NULL,
  `password` varchar(255) NOT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  UNIQUE KEY `unique_email` (`email`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8;