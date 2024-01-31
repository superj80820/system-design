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