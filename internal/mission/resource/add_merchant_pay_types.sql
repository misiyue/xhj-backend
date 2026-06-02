-- merchant 开通支付类型 JSON，如 {"hd":{"pay_type":"801"},"hm":{"pay_type":"106"}}
ALTER TABLE `merchant`
  ADD COLUMN IF NOT EXISTS `pay_types` varchar(255) DEFAULT NULL COMMENT '开通支付类型json{"hd":{"pay_type":"801"}}：hd-宏达，hm-汇美' AFTER `is_close`;
