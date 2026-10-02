-- 回退时把列加回来，评论类通知的评论 ID 取 target_id
ALTER TABLE `message` ADD COLUMN `comment_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论类通知指向的评论 ID' AFTER `video_id`;
ALTER TABLE `message_thread` ADD COLUMN `comment_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论类通知指向的评论 ID' AFTER `video_id`;

UPDATE `message` SET `comment_id` = `target_id`
WHERE `type` IN ('like_comment', 'comment', 'reply');
UPDATE `message_thread` SET `comment_id` = `target_id`
WHERE `type` IN ('like_comment', 'comment', 'reply');
