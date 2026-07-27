-- accounts 表完整建表语句
CREATE TABLE IF NOT EXISTS accounts (
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
    PRIMARY KEY (id),
    UNIQUE KEY uk_openid (openid),
    UNIQUE KEY uk_account (account),
    KEY idx_account (account)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='账号表';

