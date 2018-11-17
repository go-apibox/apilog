package apilog

// 日志表使用MyISAM，避免清理InnoDB时产生大量binlog
const MYSQL_SCHEMA_LOG = "" +
	"CREATE TABLE IF NOT EXISTS `%s` (\n" +
	"  `log_id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
	"  `request_id` VARCHAR(255) NOT NULL COMMENT '请求ID',\n" +
	"  `request_method` VARCHAR(10) NOT NULL COMMENT 'HTTP请求类型',\n" +
	"  `app_id` VARCHAR(255) NOT NULL COMMENT '调用方使用的应用ID',\n" +
	"  `action` VARCHAR(255) NOT NULL COMMENT '接口名称',\n" +
	"  `code` VARCHAR(255) NOT NULL COMMENT '调用返回代码',\n" +
	"  `ip` VARCHAR(255) NOT NULL COMMENT '调用方的IP地址',\n" +
	"  `country` VARCHAR(255) NOT NULL COMMENT '调用方所在国家',\n" +
	"  `region` VARCHAR(255) NOT NULL COMMENT '调用方所在地区',\n" +
	"  `city` VARCHAR(255) NOT NULL COMMENT '调用方所在城市',\n" +
	"  `isp` VARCHAR(255) NOT NULL COMMENT '调用方的ISP服务商',\n" +
	"  `request_time` INT UNSIGNED NOT NULL COMMENT '请求时间戳',\n" +
	"  `elapse_time` BIGINT UNSIGNED NOT NULL COMMENT '消耗时间，单位为纳秒',\n" +
	"  PRIMARY KEY (`log_id`),\n" +
	"  INDEX `idx_request_id` (`request_id` ASC),\n" +
	"  INDEX `idx_app_id` (`app_id` ASC),\n" +
	"  INDEX `idx_action` (`action` ASC),\n" +
	"  INDEX `idx_ip` (`ip` ASC),\n" +
	"  INDEX `idx_request_time` (`request_time` ASC))\n" +
	"ENGINE = MyISAM DEFAULT CHARSET=utf8 AUTO_INCREMENT=1"

const MYSQL_SCHEMA_LOG_DETAIL = "" +
	"CREATE TABLE IF NOT EXISTS `%s` (\n" +
	"  `log_id` BIGINT UNSIGNED NOT NULL,\n" +
	"  `request_id` VARCHAR(255) NOT NULL COMMENT '请求ID',\n" +
	"  `request` LONGTEXT NOT NULL COMMENT '请求原文',\n" +
	"  `response` LONGTEXT NOT NULL COMMENT '响应原文',\n" +
	"  PRIMARY KEY (`log_id`),\n" +
	"  INDEX `idx_request_id` (`request_id` ASC))\n" +
	"ENGINE = MyISAM DEFAULT CHARSET=utf8 AUTO_INCREMENT=1"

const SQLITE3_SCHEMA_LOG = "" +
	"CREATE TABLE IF NOT EXISTS `%s` (\n" +
	"	`log_id`	INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,\n" +
	"	`request_id`	TEXT NOT NULL, -- 请求ID\n" +
	"	`request_method`	TEXT NOT NULL, -- HTTP请求类型\n" +
	"	`app_id`	TEXT NOT NULL, -- 调用方使用的应用ID\n" +
	"	`action`	TEXT NOT NULL, -- 接口名称\n" +
	"	`code`	TEXT NOT NULL, -- 调用返回代码\n" +
	"	`ip`	TEXT NOT NULL, -- 调用方的IP地址\n" +
	"	`country`	TEXT NOT NULL, -- 调用方所在国家\n" +
	"	`region`	TEXT NOT NULL, -- 调用方所在地区\n" +
	"	`city`	TEXT NOT NULL, -- 调用方所在城市\n" +
	"	`isp`	TEXT NOT NULL, -- 调用方的ISP服务商\n" +
	"	`request_time`	INTEGER NOT NULL, -- 请求时间戳\n" +
	"	`elapse_time`	INTEGER NOT NULL -- 消耗时间，单位为纳秒\n" +
	");\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_request_id` ON `%s` (`request_id` ASC);\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_app_id` ON `%s` (`app_id` ASC);\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_action` ON `%s` (`action` ASC);\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_request_time` ON `%s` (`request_time` ASC);\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_ip` ON `%s` (`ip` ASC);"

const SQLITE3_SCHEMA_LOG_DETAIL = "" +
	"CREATE TABLE IF NOT EXISTS `%s` (\n" +
	"	`log_id`	INTEGER NOT NULL,\n" +
	"	`request_id`	TEXT NOT NULL, -- 请求ID\n" +
	"	`request`	TEXT NOT NULL, -- 请求原文\n" +
	"	`response`	TEXT NOT NULL, -- 响应原文\n" +
	"	PRIMARY KEY(log_id)\n" +
	");\n" +
	"CREATE INDEX IF NOT EXISTS `%s_idx_request_id` ON `%s` (`request_id` ASC);"
