-- 076_add_partner_system.sql
-- 合作伙伴管理系统：伙伴表、积分记录表、用户关联字段

CREATE TABLE IF NOT EXISTS partners (
    id BIGSERIAL PRIMARY KEY,
    partner_name VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    phone VARCHAR(50),
    referral_code VARCHAR(16) NOT NULL UNIQUE,
    pending_points DECIMAL(20,8) NOT NULL DEFAULT 0,
    withdrawn_points DECIMAL(20,8) NOT NULL DEFAULT 0,
    notes TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_partners_status ON partners(status);
CREATE INDEX IF NOT EXISTS idx_partners_email ON partners(email);

CREATE TABLE IF NOT EXISTS partner_commissions (
    id BIGSERIAL PRIMARY KEY,
    partner_id BIGINT NOT NULL REFERENCES partners(id),
    referred_user_id BIGINT NOT NULL,
    points DECIMAL(20,8) NOT NULL DEFAULT 0,
    source_cost DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pc_partner_created ON partner_commissions(partner_id, created_at);
CREATE INDEX IF NOT EXISTS idx_pc_referred ON partner_commissions(referred_user_id);

ALTER TABLE users ADD COLUMN IF NOT EXISTS partner_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_users_partner_id ON users(partner_id);
