-- Add per-partner signup bonus and max points per user fields
ALTER TABLE partners ADD COLUMN signup_bonus DECIMAL(20,8) NOT NULL DEFAULT 0;
ALTER TABLE partners ADD COLUMN max_points_per_user DECIMAL(20,8) NOT NULL DEFAULT 0;

COMMENT ON COLUMN partners.signup_bonus IS 'Gift balance awarded to new users registering via this partner code';
COMMENT ON COLUMN partners.max_points_per_user IS 'Max points earnable from a single referred user, 0 = unlimited';
