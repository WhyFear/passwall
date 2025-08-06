SELECT type,
       COUNT(CASE WHEN config LIKE '%password%' THEN 1 END)     AS 有password的节点数,
       COUNT(CASE WHEN config NOT LIKE '%password%' THEN 1 END) AS 无password的节点数
FROM proxies
GROUP BY type;


alter table proxies
    add column if not exists password varchar(255) not null default '';

drop index if exists idx_domain_port;

create unique index idx_domain_port_password
    on proxies (domain, port, password);



-- 这是一个示例 SQL 脚本，用于将历史数据迁移到新的表结构。
-- 您需要根据您的具体需求进行修改。

-- 1. 为 `proxies` 表添加 `password` 列（如果尚未添加）
-- ALTER TABLE proxies ADD COLUMN password VARCHAR(255);

-- 2. 更新 `password` 列的值
-- 对于 vmess 和 vless 类型，使用 uuid
UPDATE proxies
SET password = COALESCE((config::json) ->> 'uuid', '')
WHERE type IN ('vmess', 'vless')
  AND password = '';

-- 对于 ss 和 trojan 类型，使用 password
UPDATE proxies
SET password = COALESCE((config::json) ->> 'password', '')
WHERE type IN ('ss', 'trojan', 'hysteria2', 'http', 'socks5', 'ssr', 'anytls', 'tuic')
  AND password = '';

-- 对于其他类型，您可以根据需要添加更多的更新语句
UPDATE proxies
SET password = COALESCE((config::json) ->> 'auth_str', '')
WHERE type IN ('hysteria')
  AND password = '';

-- 3. 删除旧的唯一索引（如果尚未删除）
-- DROP INDEX IF EXISTS idx_domain_port;

-- 4. 创建新的唯一索引（如果尚未创建）
-- CREATE UNIQUE INDEX IF NOT EXISTS idx_domain_port_password ON proxies (domain, port, password);