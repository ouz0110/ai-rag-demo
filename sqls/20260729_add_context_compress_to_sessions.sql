-- nocli_sessions 增加上下文压缩控制字段
ALTER TABLE nocli_sessions
  ADD COLUMN `compress_count`         INT    NOT NULL DEFAULT 0 COMMENT '已触发上下文压缩次数',
  ADD COLUMN `last_compressed_at`     BIGINT NOT NULL DEFAULT 0 COMMENT '最近一次压缩时间戳（秒）',
  ADD COLUMN `last_checkpoint_msg_id` BIGINT NOT NULL DEFAULT 0 COMMENT '最新上下文Checkpoint消息ID';

-- nocli_messages 增加消息类型字段区分普通 System 消息与 Checkpoint 压缩节点
ALTER TABLE nocli_messages
  ADD COLUMN `msg_type` TINYINT NOT NULL DEFAULT 0 COMMENT '消息类型: 0-普通消息(含系统提示词), 1-上下文压缩Checkpoint消息';
