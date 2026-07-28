-- 计费单价规则表
CREATE TABLE IF NOT EXISTS `billing_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `provider` varchar(32) NOT NULL COMMENT '供应商: openai, cohere, local, ali',
  `service_type` varchar(32) NOT NULL COMMENT '服务类型: openai, embedding, rerank',
  `model_name` varchar(64) NOT NULL COMMENT '模型名称',
  `input_unit_price` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '输入单价(每1k单位)',
  `output_unit_price` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '输出单价(每1k单位)',
  `unit_size` int NOT NULL DEFAULT '1000' COMMENT '计费单位基数(默认1000)',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1-生效 2-失效',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_provider_type_model` (`provider`, `service_type`, `model_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI计费单价规则表';

-- 用户AI计费余额表
CREATE TABLE IF NOT EXISTS `user_balances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id` varchar(64) NOT NULL COMMENT '用户ID / 租户ID',
  `balance` decimal(18,6) NOT NULL DEFAULT '100.000000' COMMENT '当前可用余额/积分(初始化赠送100额度)',
  `gift_balance` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '赠送/活动余额',
  `total_consumed` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '历史累计消耗',
  `version` bigint unsigned NOT NULL DEFAULT '0' COMMENT '乐观锁版本号',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户AI计费余额表';

-- AI消费消耗流水明细表
CREATE TABLE IF NOT EXISTS `billing_usage_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `request_id` varchar(64) NOT NULL COMMENT '全局唯一请求ID(幂等键)',
  `user_id` varchar(64) NOT NULL COMMENT '用户ID',
  `service_type` varchar(32) NOT NULL COMMENT '服务类型: openai, embedding, rerank',
  `provider` varchar(32) NOT NULL COMMENT '供应商',
  `model_name` varchar(64) NOT NULL COMMENT '使用模型',
  `prompt_tokens` int NOT NULL DEFAULT '0' COMMENT '输入Tokens数',
  `completion_tokens` int NOT NULL DEFAULT '0' COMMENT '输出Tokens数',
  `total_tokens` int NOT NULL DEFAULT '0' COMMENT '总Tokens数',
  `doc_count` int NOT NULL DEFAULT '0' COMMENT 'Rerank文档数/图片数',
  `total_cost` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '实际扣除费用/积分',
  `status` tinyint NOT NULL DEFAULT '1' COMMENT '状态: 1-计费成功 2-部分退费 3-计费失败',
  `raw_usage_json` text COMMENT '原始Usage数据',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_created` (`user_id`, `created_at`),
  KEY `idx_type_created` (`service_type`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI消费消耗流水明细表';

-- 给 knowledge_documents 增加向量化花费字段
ALTER TABLE `knowledge_documents` ADD COLUMN `embedding_cost` decimal(18,6) NOT NULL DEFAULT '0.000000' COMMENT '文档向量化花费金额' AFTER `total_chunks`;
