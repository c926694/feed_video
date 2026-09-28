ALTER TABLE `comment` DROP INDEX `idx_comment_video_parent`;
ALTER TABLE `comment` ADD INDEX `idx_comment_video_id`(`video_id` ASC);

ALTER TABLE `comment`
  DROP COLUMN `parent_id`,
  DROP COLUMN `reply_to_id`,
  DROP COLUMN `reply_to_user_id`,
  DROP COLUMN `reply_to_user_name`,
  DROP COLUMN `reply_count`;
