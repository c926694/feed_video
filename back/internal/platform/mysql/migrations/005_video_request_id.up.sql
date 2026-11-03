-- 发布记录判重：客户端请求 ID 配合唯一索引防止重复创建
ALTER TABLE `video` ADD COLUMN `request_id` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL DEFAULT NULL COMMENT '客户端创建请求 ID，重复提交判重用';
ALTER TABLE `video` ADD UNIQUE INDEX `idx_video_author_request`(`author_id` ASC, `request_id` ASC) USING BTREE;
