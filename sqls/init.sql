-- =================================================================================
-- ai-rag-demo 全量数据库建表与初始化 DDL 脚本 (Init SQL Schema)
-- 数据库字符集: utf8mb4 / utf8mb4_general_ci
-- 合并时间: 2026-08-04
-- 包含模块: 账号系统、NoCLI 会话/消息/中断审批、RAG 知识库与切片、AI 计费规则与流水
-- =================================================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ---------------------------------------------------------------------------------
-- 1. 账号表 (Accounts Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `accounts` (
    `id`              BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键',
    `openid`          VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '用户唯一标识',
    `account`         VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '登录账号（手机号/邮箱/用户名等）',
    `password`        VARBINARY(255) NOT NULL DEFAULT '' COMMENT '密码(哈希存储)',
    `password_salt`   VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '密码加密盐',
    `nickname`        VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '昵称',
    `avatar`          VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '头像URL',
    `status`          INT           NOT NULL DEFAULT 1 COMMENT '状态 1-启用 2-禁用',
    `last_login_time` BIGINT        NOT NULL DEFAULT 0 COMMENT '最后登录时间（时间戳:秒）',
    `created_at`      BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    `updated_at`      BIGINT        NOT NULL DEFAULT 0 COMMENT '更新时间（时间戳:秒）',
    `deleted_at`      BIGINT        NOT NULL DEFAULT 0 COMMENT '删除时间（时间戳:秒）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_openid` (`openid`),
    UNIQUE KEY `uk_account` (`account`),
    KEY `idx_account` (`account`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='账号表';

-- ---------------------------------------------------------------------------------
-- 2. NoCLI 会话表 (NoCLI Sessions Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `nocli_sessions` (
    `id`                      BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键',
    `session_id`              VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '会话唯一标识',
    `openid`                  VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '用户openid',
    `name`                    VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '会话名称',
    `status`                  INT           NOT NULL DEFAULT 1 COMMENT '会话状态: 1-IDLE, 2-RUNNING, 3-INTERRUPTED',
    `compress_count`         INT           NOT NULL DEFAULT 0 COMMENT '已触发上下文压缩次数',
    `last_compressed_at`     BIGINT        NOT NULL DEFAULT 0 COMMENT '最近一次压缩时间戳（秒）',
    `last_checkpoint_msg_id` BIGINT        NOT NULL DEFAULT 0 COMMENT '最新上下文Checkpoint消息ID',
    `created_at`              BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    `updated_at`              BIGINT        NOT NULL DEFAULT 0 COMMENT '更新时间（时间戳:秒）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_session_id` (`session_id`),
    KEY `idx_openid` (`openid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli会话表';

-- ---------------------------------------------------------------------------------
-- 3. NoCLI 消息表 (NoCLI Messages Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `nocli_messages` (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
    `session_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '会话ID',
    `msg`        TEXT         NOT NULL COMMENT '消息内容（JSON字符串）',
    `msg_type`   TINYINT      NOT NULL DEFAULT 0 COMMENT '消息类型: 0-普通消息(含系统提示词), 1-上下文压缩Checkpoint消息',
    `created_at` BIGINT       NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    PRIMARY KEY (`id`),
    KEY `idx_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli消息表';

-- ---------------------------------------------------------------------------------
-- 4. NoCLI 中断审批表 (NoCLI Interrupts Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `nocli_interrupts` (
    `id`             BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键',
    `interrupt_id`   VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '中断事件唯一标识',
    `session_id`     VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '关联会话ID',
    `status`         INT           NOT NULL DEFAULT 1 COMMENT '中断状态: 1-PENDING, 2-APPROVED, 3-REJECTED, 4-EXPIRED',
    `tool_call_id`   VARCHAR(128)  NOT NULL DEFAULT '' COMMENT 'OpenAI ToolCallID',
    `tool_name`      VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '待执行的工具名称',
    `arguments`      TEXT          NOT NULL COMMENT '待执行的工具参数(JSON)',
    `approve_scope`  INT           NOT NULL DEFAULT 1 COMMENT '授权范围: 1-SINGLE_CALL, 2-SESSION_TOOL',
    `reject_reason`  VARCHAR(512)  NOT NULL DEFAULT '' COMMENT '拒绝原因/用户意见',
    `handler_openid` VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '处理人openid',
    `created_at`     BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    `handled_at`     BIGINT        NOT NULL DEFAULT 0 COMMENT '处理时间（时间戳:秒）',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_interrupt_id` (`interrupt_id`),
    KEY `idx_session_status` (`session_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli中断审批表';

-- ---------------------------------------------------------------------------------
-- 5. 知识库主表 (Knowledge Base Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `knowledge_bases` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`    VARCHAR(64)  NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `kb_id`        VARCHAR(64)  NOT NULL COMMENT '知识库UUID标识',
  `name`         VARCHAR(128) NOT NULL COMMENT '知识库名称',
  `description`  VARCHAR(512) DEFAULT '' COMMENT '知识库描述',
  `is_default`   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否为系统默认公共知识库 (0:否, 1:是)',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_kb_id` (`kb_id`),
  KEY `idx_tenant_kb` (`tenant_id`, `is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库主表';

-- ---------------------------------------------------------------------------------
-- 6. 知识库文档表 (Knowledge Document Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `knowledge_documents` (
  `id`                BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`         VARCHAR(64)   NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `kb_id`             VARCHAR(64)   NOT NULL DEFAULT 'kb_default_system' COMMENT '关联知识库UUID标识',
  `collection_id`     BIGINT        NOT NULL DEFAULT 0 COMMENT '所属知识库Collection ID',
  `doc_id`            VARCHAR(64)   NOT NULL COMMENT '业务文档UUID',
  `title`             VARCHAR(255)  NOT NULL COMMENT '文档标题',
  `source_type`       VARCHAR(32)   NOT NULL DEFAULT 'txt' COMMENT '文档类型 (pdf, docx, md, txt, csv, json)',
  `doc_version`       VARCHAR(32)   NOT NULL DEFAULT 'v1.0' COMMENT '文档版本号',
  `category`          VARCHAR(64)   NOT NULL DEFAULT 'default' COMMENT '文档业务分类',
  `tags`              VARCHAR(512)  DEFAULT '' COMMENT '文档标签(逗号分隔)',
  `is_active`         TINYINT(1)    NOT NULL DEFAULT 1 COMMENT '是否失效 (0:失效/作废, 1:生效中)',
  `supersedes_doc_id` VARCHAR(64)   DEFAULT '' COMMENT '被替代的旧文档UUID',
  `source_url`        VARCHAR(1024) DEFAULT '' COMMENT '原始文件存储地址 (OSS URL)',
  `file_path`         VARCHAR(512)  DEFAULT '' COMMENT '文件磁盘绝对路径',
  `file_hash`         VARCHAR(64)   DEFAULT '' COMMENT '文件内容 SHA256 哈希值 (用于增量去重对比)',
  `status`            TINYINT       NOT NULL DEFAULT 0 COMMENT '处理状态 (0:待处理, 1:解析中, 2:已向量化, 3:失败)',
  `total_chunks`      INT           NOT NULL DEFAULT 0 COMMENT '总切片数',
  `embedding_cost`    DECIMAL(18,6) NOT NULL DEFAULT '0.000000' COMMENT '文档向量化花费金额',
  `err_msg`           TEXT          COMMENT '解析/向量化失败异常信息',
  `created_at`        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`        DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_id` (`doc_id`),
  KEY `idx_tenant_doc` (`tenant_id`),
  KEY `idx_kb_doc` (`kb_id`),
  KEY `idx_file_path` (`file_path`(255)),
  KEY `idx_file_hash` (`file_hash`),
  KEY `idx_active_kb` (`tenant_id`, `kb_id`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库文档表';

-- ---------------------------------------------------------------------------------
-- 7. 知识库文档切片表 (Knowledge Chunk Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `knowledge_chunks` (
  `id`            BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`     VARCHAR(64)   NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `doc_id`        VARCHAR(64)   NOT NULL COMMENT '所属文档UUID',
  `chunk_id`      VARCHAR(64)   NOT NULL COMMENT '切片UUID (对应向量库 Vector ID)',
  `parent_id`     VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '父块UUID (为空表示自身为粗粒度父块)',
  `h1`            VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '一级标题',
  `h2`            VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '二级标题',
  `h3`            VARCHAR(255)  NOT NULL DEFAULT '' COMMENT '三级标题',
  `start_line`    INT           NOT NULL DEFAULT 0 COMMENT '起始行号',
  `end_line`      INT           NOT NULL DEFAULT 0 COMMENT '结束行号',
  `has_table`     TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否包含表格 (0:否, 1:是)',
  `has_code`      TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否包含代码块 (0:否, 1:是)',
  `chunk_index`   INT           NOT NULL DEFAULT 0 COMMENT '切片全局顺序序号',
  `chunk_hash`    VARCHAR(64)   DEFAULT '' COMMENT '切片内容 SHA256 哈希 (用于增量 Diff 对比)',
  `chunk_type`    VARCHAR(32)   NOT NULL DEFAULT 'text' COMMENT '切片类型 (parent:父块, text:文本, table:表格, code:代码)',
  `content`       MEDIUMTEXT    NOT NULL COMMENT '文本切片内容',
  `token_count`   INT           NOT NULL DEFAULT 0 COMMENT 'Token / 字符消耗数量',
  `vector_status` TINYINT       NOT NULL DEFAULT 0 COMMENT '向量同步状态 (0:未同步, 1:已同步)',
  `is_active`     TINYINT(1)    NOT NULL DEFAULT 1 COMMENT '切片是否生效 (0:失效, 1:生效)',
  `created_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chunk_id` (`chunk_id`),
  KEY `idx_tenant_chunk` (`tenant_id`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_active_doc` (`doc_id`, `is_active`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库文档切片表';

-- ---------------------------------------------------------------------------------
-- 8. AI 计费单价规则表 (Billing Rules Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `billing_rules` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `provider`          VARCHAR(32)     NOT NULL COMMENT '供应商: openai, cohere, local, ali',
  `service_type`      VARCHAR(32)     NOT NULL COMMENT '服务类型: openai, embedding, rerank',
  `model_name`        VARCHAR(64)     NOT NULL COMMENT '模型名称',
  `input_unit_price`  DECIMAL(18,6)   NOT NULL DEFAULT '0.000000' COMMENT '输入单价(每1k单位)',
  `output_unit_price` DECIMAL(18,6)   NOT NULL DEFAULT '0.000000' COMMENT '输出单价(每1k单位)',
  `unit_size`         INT             NOT NULL DEFAULT '1000' COMMENT '计费单位基数(默认1000)',
  `status`            TINYINT         NOT NULL DEFAULT '1' COMMENT '状态: 1-生效 2-失效',
  `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_provider_type_model` (`provider`, `service_type`, `model_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI计费单价规则表';

-- ---------------------------------------------------------------------------------
-- 9. 用户 AI 计费余额表 (User Balances Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `user_balances` (
  `id`             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `user_id`        VARCHAR(64)     NOT NULL COMMENT '用户ID / 租户ID',
  `balance`        DECIMAL(18,6)   NOT NULL DEFAULT '100.000000' COMMENT '当前可用余额/积分(初始化赠送100额度)',
  `gift_balance`   DECIMAL(18,6)   NOT NULL DEFAULT '0.000000' COMMENT '赠送/活动余额',
  `total_consumed` DECIMAL(18,6)   NOT NULL DEFAULT '0.000000' COMMENT '历史累计消耗',
  `version`        BIGINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '乐观锁版本号',
  `created_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`     DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户AI计费余额表';

-- ---------------------------------------------------------------------------------
-- 10. AI 消费消耗流水明细表 (Billing Usage Logs Table)
-- ---------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS `billing_usage_logs` (
  `id`                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `request_id`        VARCHAR(64)     NOT NULL COMMENT '全局唯一请求ID(幂等键)',
  `user_id`           VARCHAR(64)     NOT NULL COMMENT '用户ID',
  `service_type`      VARCHAR(32)     NOT NULL COMMENT '服务类型: openai, embedding, rerank',
  `provider`          VARCHAR(32)     NOT NULL COMMENT '供应商',
  `model_name`        VARCHAR(64)     NOT NULL COMMENT '使用模型',
  `prompt_tokens`     INT             NOT NULL DEFAULT '0' COMMENT '输入Tokens数',
  `completion_tokens` INT             NOT NULL DEFAULT '0' COMMENT '输出Tokens数',
  `total_tokens`      INT             NOT NULL DEFAULT '0' COMMENT '总Tokens数',
  `doc_count`         INT             NOT NULL DEFAULT '0' COMMENT 'Rerank文档数/图片数',
  `total_cost`        DECIMAL(18,6)   NOT NULL DEFAULT '0.000000' COMMENT '实际扣除费用/积分',
  `status`            TINYINT         NOT NULL DEFAULT '1' COMMENT '状态: 1-计费成功 2-部分退费 3-计费失败',
  `raw_usage_json`    TEXT            COMMENT '原始Usage数据',
  `created_at`        DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '记录时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_user_created` (`user_id`, `created_at`),
  KEY `idx_type_created` (`service_type`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI消费消耗流水明细表';

SET FOREIGN_KEY_CHECKS = 1;
