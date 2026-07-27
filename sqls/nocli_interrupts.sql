-- nocli_interrupts 表完整建表语句
CREATE TABLE IF NOT EXISTS nocli_interrupts (
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
    PRIMARY KEY (id),
    UNIQUE KEY uk_interrupt_id (interrupt_id),
    KEY idx_session_status (session_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli中断审批表';

-- 数据库增量变更 ALTER 语句（针对已存在 nocli_interrupts 旧表的情况）
-- 执行说明：给 nocli_interrupts 表新增 approve_scope 授权范围字段
ALTER TABLE nocli_interrupts 
ADD COLUMN `approve_scope` INT NOT NULL DEFAULT 1 COMMENT '授权范围: 1-SINGLE_CALL, 2-SESSION_TOOL' AFTER `arguments`;
