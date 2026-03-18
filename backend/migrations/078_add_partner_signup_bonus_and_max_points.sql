-- Add per-partner signup bonus and max points per user fields
ALTER TABLE partners ADD COLUMN signup_bonus DECIMAL(20,8) NOT NULL DEFAULT 0;
ALTER TABLE partners ADD COLUMN max_points_per_user DECIMAL(20,8) NOT NULL DEFAULT 0;

-- Backfill: copy global partner_signup_bonus to all existing partners so they keep
-- the same behavior they had before per-partner settings were introduced.
UPDATE partners
SET signup_bonus = COALESCE(
    (SELECT CAST(value AS DECIMAL(20,8))
     FROM settings
     WHERE key = 'partner_signup_bonus' AND value <> '' AND value <> '0'),
    0
);

COMMENT ON COLUMN partners.signup_bonus IS 'Gift balance awarded to new users registering via this partner code';
COMMENT ON COLUMN partners.max_points_per_user IS 'Max points earnable from a single referred user, 0 = unlimited';
