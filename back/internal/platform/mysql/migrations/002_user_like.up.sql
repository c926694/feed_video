-- 点赞关系表，target_type 取值 video / comment

CREATE TABLE IF NOT EXISTS `user_like`  (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '点赞关系 ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '点赞用户 ID',
  `target_type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '点赞目标类型，video 或 comment',
  `target_id` bigint UNSIGNED NOT NULL COMMENT '点赞目标 ID',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_like_user_target`(`user_id` ASC, `target_type` ASC, `target_id` ASC) USING BTREE,
  INDEX `idx_like_target`(`target_type` ASC, `target_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci ROW_FORMAT = Dynamic;
