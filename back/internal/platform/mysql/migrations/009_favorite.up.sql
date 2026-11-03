-- 收藏功能：user_favorite 关系表与 video 收藏数冗余列
CREATE TABLE IF NOT EXISTS `user_favorite` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '收藏关系 ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '收藏用户 ID',
  `video_id` bigint UNSIGNED NOT NULL COMMENT '视频 ID',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '收藏时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `idx_favorite_user_video`(`user_id` ASC, `video_id` ASC) USING BTREE,
  INDEX `idx_favorite_video`(`video_id` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci ROW_FORMAT = Dynamic;

ALTER TABLE `video` ADD COLUMN `favorite_count` bigint NOT NULL DEFAULT 0 COMMENT '收藏数，冗余计数';
