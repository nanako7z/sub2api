-- partner_commissions.partner_id 改为可为 NULL
-- 删除 partner 时，关联积分记录的 partner_id 自动置 NULL（保留历史审计）

-- 1. 允许 partner_id 为 NULL
ALTER TABLE partner_commissions ALTER COLUMN partner_id DROP NOT NULL;

-- 2. 删除自动命名的外键约束（PostgreSQL 默认命名为 partner_commissions_partner_id_fkey）
ALTER TABLE partner_commissions DROP CONSTRAINT IF EXISTS partner_commissions_partner_id_fkey;

-- 3. 重新添加外键，附加 ON DELETE SET NULL
ALTER TABLE partner_commissions
    ADD CONSTRAINT partner_commissions_partner_id_fkey
    FOREIGN KEY (partner_id) REFERENCES partners(id) ON DELETE SET NULL;
