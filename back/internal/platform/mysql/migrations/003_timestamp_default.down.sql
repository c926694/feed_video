ALTER TABLE `comment` MODIFY COLUMN `created_at` datetime(3) NOT NULL,
  MODIFY COLUMN `updated_at` datetime(3) NOT NULL;

ALTER TABLE `follow` MODIFY COLUMN `created_at` datetime(3) NOT NULL;

ALTER TABLE `user` MODIFY COLUMN `created_at` datetime(3) NOT NULL,
  MODIFY COLUMN `updated_at` datetime(3) NOT NULL;

ALTER TABLE `video` MODIFY COLUMN `created_at` datetime(3) NULL DEFAULT NULL,
  MODIFY COLUMN `updated_at` datetime(3) NOT NULL;
