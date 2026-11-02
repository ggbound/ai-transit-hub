-- 虚拟 Agent 任务表（/v1/agent/tasks 宿主机全流程线路）
-- 执行方式：mysql -u <user> -p <database> < migrations/2026_10_23_create_agent_tasks.sql

CREATE TABLE IF NOT EXISTS `agent_tasks` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `task_id` VARCHAR(64) NOT NULL COMMENT '对外任务ID（agt_ 前缀）',
    `user_id` BIGINT UNSIGNED NOT NULL COMMENT '发起用户ID',
    `api_key_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '发起时使用的用户 API Key ID',
    `model_id` BIGINT UNSIGNED NOT NULL COMMENT '使用的模型ID',
    `provider_id` BIGINT UNSIGNED NOT NULL COMMENT '模型所属服务商ID',
    `task` TEXT NOT NULL COMMENT '任务指令（自然语言）',
    `skill_ids` JSON DEFAULT NULL COMMENT '装载的技能ID列表',
    `max_iterations` INT NOT NULL DEFAULT 100 COMMENT 'Agent 循环轮数上限',
    `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/running/completed/partial/failed',
    `iterations` INT NOT NULL DEFAULT 0 COMMENT '实际执行轮数',
    `result` MEDIUMTEXT COMMENT '最终文本结果',
    `error_message` TEXT COMMENT '失败原因',
    `skills_used` JSON DEFAULT NULL COMMENT '技能使用详情（装载了哪些 SKILL.md/附件）',
    `assets` JSON DEFAULT NULL COMMENT '资产清单（name/size/mime/download_url）',
    `prompt_tokens` INT NOT NULL DEFAULT 0 COMMENT '提示 token 累计',
    `completion_tokens` INT NOT NULL DEFAULT 0 COMMENT '补全 token 累计',
    `total_tokens` INT NOT NULL DEFAULT 0 COMMENT '总 token 累计',
    `cost` DECIMAL(10,6) NOT NULL DEFAULT 0 COMMENT '总成本',
    `started_at` TIMESTAMP NULL DEFAULT NULL COMMENT '开始执行时间',
    `completed_at` TIMESTAMP NULL DEFAULT NULL COMMENT '执行完成时间',
    `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_agent_tasks_task_id` (`task_id`),
    KEY `idx_agent_tasks_user_id` (`user_id`),
    KEY `idx_agent_tasks_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='虚拟 Agent 任务';
