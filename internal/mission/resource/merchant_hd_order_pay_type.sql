-- merchant_hd_order 增加 pay_type 通道字段
ALTER TABLE `merchant_hd_order`
  ADD COLUMN IF NOT EXISTS `pay_type` varchar(8) NOT NULL DEFAULT '' COMMENT '支付类型：801-小额，802-中额，803-大额' AFTER `local_no`;
