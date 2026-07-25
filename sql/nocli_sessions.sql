-- nocli_sessions 表完整建表语句
CREATE TABLE IF NOT EXISTS nocli_sessions (
    `id`           BIGINT        NOT NULL AUTO_INCREMENT COMMENT '主键',
    `session_id`   VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '会话唯一标识',
    `openid`       VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '用户openid',
    `name`         VARCHAR(128)  NOT NULL DEFAULT '' COMMENT '会话名称',
    `status`       INT           NOT NULL DEFAULT 1 COMMENT '会话状态: 1-IDLE, 2-RUNNING, 3-INTERRUPTED',
    `created_at`   BIGINT        NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    `updated_at`   BIGINT        NOT NULL DEFAULT 0 COMMENT '更新时间（时间戳:秒）',
    PRIMARY KEY (id),
    UNIQUE KEY uk_session_id (session_id),
    KEY idx_openid (openid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli会话表';
