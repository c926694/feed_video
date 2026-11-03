-- 关注与粉丝列表要按"最近在前"翻页：
-- 关注列表按 follower 过滤后按 id 倒序，粉丝列表按 following 过滤后按 id 倒序，
-- 两条复合索引让筛人与排序一次走完
ALTER TABLE `follow` ADD INDEX `idx_follow_follower`(`follower` ASC, `id` ASC) USING BTREE;
ALTER TABLE `follow` ADD INDEX `idx_follow_following`(`following` ASC, `id` ASC) USING BTREE;
