ALTER TABLE `comment` ADD COLUMN `reply_count` bigint NOT NULL DEFAULT 0 COMMENT '子评论数，冗余计数';
