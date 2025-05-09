CREATE TABLE `order` (
                         `id` bigint unsigned NOT NULL AUTO_INCREMENT,
                         `order_sn` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
                         `receiver` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
                         `remark` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
                         `status` bigint NOT NULL,
                         `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
                         `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                         PRIMARY KEY (`id`),
                         UNIQUE KEY `uniq_order_sn` (`order_sn`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;



CREATE TABLE `photo` (
                         `id` bigint unsigned NOT NULL AUTO_INCREMENT,
                         `order_id` bigint unsigned NOT NULL,
                         `url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
                         `size` bigint NOT NULL,
                         `unit` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL,
                         `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
                         `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
                         PRIMARY KEY (`id`),
                         KEY `idx_order_id` (`order_id`)
) ENGINE=InnoDB AUTO_INCREMENT=45 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
