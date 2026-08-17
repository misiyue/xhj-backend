-- 可选：手工执行；亦可通过 `lumenim migrate` 的 AutoMigrate 同步
ALTER TABLE `merchant`
  ADD COLUMN `surety_bill_id` int(11) NULL DEFAULT 0 COMMENT '保证金冻结凭证id' AFTER `surety`;
