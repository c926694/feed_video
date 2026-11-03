-- 视频发布状态：created 已创建（文件上传中）、published 已发布、failed 上传失败
ALTER TABLE `video` ADD COLUMN `status` varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'published' COMMENT '发布状态，created 已创建（文件上传中）、published 已发布、failed 上传失败';
