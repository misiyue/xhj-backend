-- 可选：手动执行（亦可通过 `lumenim migrate` 的 GORM AutoMigrate 自动加列）
ALTER TABLE `users`
  ADD COLUMN `device_code` varchar(128) NULL COMMENT '设备码' AFTER `invite_user_id`;
