-- 075: Add referral system
-- Adds gift_balance, referral_code, referrer_id to users table
-- Creates referral_commissions table for tracking commission events

-- Add gift_balance and referral fields to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS gift_balance decimal(20,8) NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code varchar(16);
ALTER TABLE users ADD COLUMN IF NOT EXISTS referrer_id bigint REFERENCES users(id) ON DELETE SET NULL;

-- Partial unique index for referral_code (only non-null, non-deleted)
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_referral_code_unique
  ON users (referral_code) WHERE referral_code IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_referrer_id ON users (referrer_id) WHERE referrer_id IS NOT NULL;

-- Referral commission tracking table
CREATE TABLE IF NOT EXISTS referral_commissions (
  id bigserial PRIMARY KEY,
  referrer_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  referred_user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount decimal(20,8) NOT NULL DEFAULT 0,
  source_cost decimal(20,8) NOT NULL DEFAULT 0,
  commission_rate double precision NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_referral_commissions_referrer ON referral_commissions (referrer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_referral_commissions_referred ON referral_commissions (referred_user_id);
CREATE INDEX IF NOT EXISTS idx_referral_commissions_cap ON referral_commissions (referrer_id, referred_user_id);
