-- SKILL 管理模块建表脚本（管理后台 - SKILL.md 技能包管理）
-- 执行方式：mysql -u <user> -p <database> < migrations/2026_10_23_create_skills_tables.sql

CREATE TABLE IF NOT EXISTS `skills` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(64) NOT NULL COMMENT '技能标识（frontmatter name，小写字母/数字/连字符）',
    `description` VARCHAR(1024) DEFAULT NULL COMMENT '技能描述（frontmatter description）',
    `version` VARCHAR(32) DEFAULT '0.0.1' COMMENT '技能版本',
    `license` VARCHAR(64) DEFAULT NULL COMMENT '许可证',
    `allowed_tools` TEXT COMMENT '允许使用的工具（逗号分隔）',
    `content` MEDIUMTEXT NOT NULL COMMENT 'SKILL.md 全文内容',
    `frontmatter` JSON DEFAULT NULL COMMENT '原始 frontmatter 元数据',
    `tags` JSON DEFAULT NULL COMMENT '自定义标签',
    `file_count` INT NOT NULL DEFAULT 0 COMMENT '附件数量（不含 SKILL.md）',
    `storage_path` VARCHAR(512) DEFAULT NULL COMMENT '附件在磁盘上的存储目录',
    `enabled` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用',
    `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人用户ID',
    `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_skills_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SKILL 技能包';

CREATE TABLE IF NOT EXISTS `skill_files` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `skill_id` BIGINT UNSIGNED NOT NULL COMMENT '所属技能ID',
    `path` VARCHAR(512) NOT NULL COMMENT '包内相对路径',
    `size` BIGINT NOT NULL DEFAULT 0 COMMENT '文件大小（字节）',
    `mime_type` VARCHAR(128) DEFAULT NULL COMMENT 'MIME 类型',
    `created_at` TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_skill_files_skill_id` (`skill_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SKILL 附件文件索引';
