-- 评论两级结构：parent_id 为 0 表示顶级评论，子评论挂在顶级评论下。
-- reply_to_* 记录被回复的对象，用于 @ 展示
ALTER TABLE `comment`
  ADD COLUMN `parent_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '父评论 ID，0 表示顶级评论',
  ADD COLUMN `reply_to_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '被回复的评论 ID，用于 @ 展示',
  ADD COLUMN `reply_to_user_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '被回复的用户 ID',
  ADD COLUMN `reply_to_user_name` varchar(64) NULL DEFAULT NULL COMMENT '被回复者昵称快照，创建时写入，不随改名刷新',
  ADD COLUMN `reply_count` bigint NOT NULL DEFAULT 0 COMMENT '子评论数，冗余计数';

ALTER TABLE `comment` DROP INDEX `idx_comment_video_id`;
ALTER TABLE `comment` ADD INDEX `idx_comment_video_parent`(`video_id` ASC, `parent_id` ASC, `created_at` ASC, `id` ASC);
