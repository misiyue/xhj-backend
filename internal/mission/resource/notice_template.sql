-- 通知模板 notice_template（亦可由 lumenim migrate AutoMigrate 建表）
CREATE TABLE IF NOT EXISTS `notice_template` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `flag` varchar(32) DEFAULT NULL COMMENT '标识：userChat-用户聊天，c2cChat-c2c聊天，c2cPaid-c2c支付，c2cTrans-c2c放币，contactApply-好友申请，groupApply-群申请',
  `title` varchar(255) DEFAULT NULL COMMENT '标题',
  `subtitle` varchar(255) DEFAULT NULL COMMENT '子标题',
  `content` varchar(255) DEFAULT NULL COMMENT '描述内容',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `flag_idx` (`flag`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO `notice_template` (`flag`, `title`, `subtitle`, `content`) VALUES
('userChat', '新消息', '', '您有一条新的聊天消息'),
('c2cChat', '新消息', '', '您有一条新的C2C聊天消息'),
('c2cPaid', '订单已支付', '', '您的订单{#order}已支付'),
('c2cTrans', '订单已放币', '', '您的订单已放币'),
('contactApply', '好友申请', '', '{#sender}请求添加您为好友'),
('groupApply', '群申请', '', '{#sender}申请加入{#group}')
ON DUPLICATE KEY UPDATE
  `title` = VALUES(`title`),
  `subtitle` = VALUES(`subtitle`),
  `content` = VALUES(`content`);
