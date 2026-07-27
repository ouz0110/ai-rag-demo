-- =================================================================================
-- 生产级 RAG 知识库系统 数据库建表 DDL
-- 数据库字符集: utf8mb4 / utf8mb4_general_ci
-- 创建时间: 2026-07-27
-- =================================================================================

-- 1. 知识库主表 (Knowledge Base Table)
-- 隔离系统默认公共知识库与用户自定义知识库
CREATE TABLE IF NOT EXISTS `knowledge_bases` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`    VARCHAR(64)  NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `user_id`      BIGINT       NOT NULL DEFAULT 0 COMMENT '创建用户ID',
  `kb_id`        VARCHAR(64)  NOT NULL COMMENT '知识库UUID标识',
  `name`         VARCHAR(128) NOT NULL COMMENT '知识库名称',
  `description`  VARCHAR(512) DEFAULT '' COMMENT '知识库描述',
  `is_default`   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否为系统默认公共知识库 (0:否, 1:是)',
  `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_kb_id` (`kb_id`),
  KEY `idx_tenant_kb` (`tenant_id`, `is_default`, `user_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库主表';


-- 2. 知识库文档表 (Knowledge Document Table)
-- 记录接入与上传的文件元数据、哈希值及状态
CREATE TABLE IF NOT EXISTS `knowledge_documents` (
  `id`            BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`     VARCHAR(64)   NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `kb_id`         VARCHAR(64)   NOT NULL DEFAULT 'kb_default_system' COMMENT '关联知识库UUID标识',
  `collection_id` BIGINT        NOT NULL DEFAULT 0 COMMENT '所属知识库Collection ID',
  `doc_id`        VARCHAR(64)   NOT NULL COMMENT '业务文档UUID',
  `title`         VARCHAR(255)  NOT NULL COMMENT '文档标题',
  `source_type`   VARCHAR(32)   NOT NULL DEFAULT 'txt' COMMENT '文档类型 (pdf, docx, md, txt, csv, json)',
  `source_url`    VARCHAR(1024) DEFAULT '' COMMENT '原始文件存储地址 (OSS URL)',
  `file_path`     VARCHAR(512)  DEFAULT '' COMMENT '文件磁盘绝对路径',
  `file_hash`     VARCHAR(64)   DEFAULT '' COMMENT '文件内容 SHA256 哈希值 (用于增量去重对比)',
  `status`        TINYINT       NOT NULL DEFAULT 0 COMMENT '处理状态 (0:待处理, 1:解析中, 2:已向量化, 3:失败)',
  `total_chunks`  INT           NOT NULL DEFAULT 0 COMMENT '总切片数',
  `err_msg`       TEXT          COMMENT '解析/向量化失败异常信息',
  `created_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_id` (`doc_id`),
  KEY `idx_tenant_doc` (`tenant_id`),
  KEY `idx_kb_doc` (`kb_id`),
  KEY `idx_file_path` (`file_path`(255)),
  KEY `idx_file_hash` (`file_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库文档表';


-- 3. 知识库文档切片表 (Knowledge Chunk Table)
-- 存储 Parent 粗粒度上下文及 Child 切片映射
CREATE TABLE IF NOT EXISTS `knowledge_chunks` (
  `id`            BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `tenant_id`     VARCHAR(64)   NOT NULL DEFAULT 'default_tenant' COMMENT '租户ID',
  `doc_id`        VARCHAR(64)   NOT NULL COMMENT '所属文档UUID',
  `chunk_id`      VARCHAR(64)   NOT NULL COMMENT '切片UUID (对应向量库 Vector ID)',
  `parent_id`     VARCHAR(64)   NOT NULL DEFAULT '' COMMENT '父块UUID (为空表示自身为粗粒度父块)',
  `chunk_index`   INT           NOT NULL DEFAULT 0 COMMENT '切片全局顺序序号',
  `content`       MEDIUMTEXT    NOT NULL COMMENT '文本切片内容',
  `token_count`   INT           NOT NULL DEFAULT 0 COMMENT 'Token / 字符消耗数量',
  `vector_status` TINYINT       NOT NULL DEFAULT 0 COMMENT '向量同步状态 (0:未同步, 1:已同步)',
  `created_at`    DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_chunk_id` (`chunk_id`),
  KEY `idx_tenant_chunk` (`tenant_id`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_parent_id` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='知识库文档切片表';
