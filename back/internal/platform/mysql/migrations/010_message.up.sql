-- 通知明细表：一笔互动一行。唯一键保证同一个人对同一个目标只通知一次，
-- 取消再点赞、消息重投、位点重放都不会产生第二条
CREATE TABLE IF NOT EXISTS `message`  (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '通知 ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '接收者 ID',
  `thread_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属会话 ID',
  `type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'like_video / like_comment / comment / reply / follow',
  `target_id` bigint UNSIGNED NOT NULL COMMENT '动作作用的对象 ID',
  `actor_id` bigint UNSIGNED NOT NULL COMMENT '触发者 ID',
  `video_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '跳转目标所属视频 ID',
  `comment_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论类通知指向的评论 ID',
  `content` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '评论原文',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_message_user_actor`(`user_id` ASC, `type` ASC, `target_id` ASC, `actor_id` ASC) USING BTREE,
  INDEX `idx_message_thread`(`thread_id` ASC, `created_at` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci ROW_FORMAT = Dynamic COMMENT = '通知明细';

-- 通知会话表：列表、未读、已读都以它为准，一条会话一行。
-- window_bucket 是 8 小时一个桶，同一目标的互动在同一个桶内累加到同一行
CREATE TABLE IF NOT EXISTS `message_thread`  (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '会话 ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '接收者 ID',
  `type` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '与明细的 type 一致',
  `target_id` bigint UNSIGNED NOT NULL COMMENT '动作作用的对象 ID',
  `window_bucket` int UNSIGNED NOT NULL COMMENT '会话窗口：unix 秒 / 28800，8 小时一个桶',
  `video_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '跳转目标所属视频 ID',
  `comment_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论类通知指向的评论 ID',
  `actor_count` int UNSIGNED NOT NULL DEFAULT 1 COMMENT '参与人数',
  `actor_ids` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '最近三个参与者 ID，逗号分隔',
  `content` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '评论类最新一条摘要',
  `is_read` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否已读',
  `read_at` datetime(3) NULL COMMENT '第一次标记已读的时间',
  `last_at` datetime(3) NOT NULL COMMENT '最后一次互动时间，列表按它倒序',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '会话首次产生时间',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE INDEX `uk_thread_user_target`(`user_id` ASC, `type` ASC, `target_id` ASC, `window_bucket` ASC) USING BTREE,
  INDEX `idx_thread_user_last`(`user_id` ASC, `last_at` ASC, `id` ASC) USING BTREE,
  INDEX `idx_thread_user_read_last`(`user_id` ASC, `is_read` ASC, `last_at` ASC) USING BTREE,
  INDEX `idx_thread_read_last`(`is_read` ASC, `last_at` ASC) USING BTREE
) ENGINE = InnoDB CHARACTER SET = utf8mb4 COLLATE = utf8mb4_unicode_ci ROW_FORMAT = Dynamic COMMENT = '通知会话';
