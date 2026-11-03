-- comment_id 与 target_id 在评论类通知里取值相同，按 type 就能推导，
-- 去掉这一列避免两个值不一致。评论类通知的评论 ID 就是 target_id
ALTER TABLE `message` DROP COLUMN `comment_id`;
ALTER TABLE `message_thread` DROP COLUMN `comment_id`;
