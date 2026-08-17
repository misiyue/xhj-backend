-- 商户会话：每人一条（inviter_id=自己, friend_id=对方），成对共享 session_id
ALTER TABLE `merchant_session`
  DROP INDEX IF EXISTS `uk_merchant_session_participants`,
  DROP INDEX IF EXISTS `uk_merchant_session_order_tag`;

ALTER TABLE `merchant_session`
  ADD COLUMN IF NOT EXISTS `session_id` int(11) NOT NULL DEFAULT 0 COMMENT '成对会话共享id' AFTER `id`;

CREATE INDEX IF NOT EXISTS `idx_merchant_session_owner_peer` ON `merchant_session` (`inviter_id`, `friend_id`, `delete_time`);
