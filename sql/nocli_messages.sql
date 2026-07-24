-- nocli_messages 表完整建表语句
CREATE TABLE IF NOT EXISTS nocli_messages (
    `id`         BIGINT       NOT NULL AUTO_INCREMENT COMMENT '主键',
    `session_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '会话ID',
    `msg`        TEXT         NOT NULL COMMENT '消息内容（JSON字符串）',
    `created_at` BIGINT       NOT NULL DEFAULT 0 COMMENT '创建时间（时间戳:秒）',
    PRIMARY KEY (id),
    KEY idx_session_id (session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='nocli消息表';
