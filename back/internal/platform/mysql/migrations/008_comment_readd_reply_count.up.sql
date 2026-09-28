-- 回复数冗余列，由写路径事务同步维护，读取时直接返回
ALTER TABLE `comment` ADD COLUMN `reply_count` bigint NOT NULL DEFAULT 0 COMMENT '子评论数，冗余计数';
