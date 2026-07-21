-- 第三方下单失败日志
CREATE TABLE IF NOT EXISTS `merchant_order_err_log` (
  `no` varchar(64) NOT NULL COMMENT '订单号',
  `request` text COMMENT '请求参数',
  `respond` text COMMENT '响应结果',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `url` varchar(255) DEFAULT NULL COMMENT '请求地址'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
