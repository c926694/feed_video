-- 回复总数不做冗余计数：由读取时多取一条判断是否还有更多
ALTER TABLE `comment` DROP COLUMN `reply_count`;
