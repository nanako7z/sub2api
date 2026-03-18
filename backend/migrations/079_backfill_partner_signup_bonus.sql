-- Backfill: copy global partner_signup_bonus to all existing partners so they keep
-- the same behavior they had before per-partner settings were introduced.
UPDATE partners
SET signup_bonus = COALESCE(
    (SELECT CAST(value AS DECIMAL(20,8))
     FROM settings
     WHERE key = 'partner_signup_bonus' AND value <> '' AND value <> '0'),
    0
)
WHERE signup_bonus = 0;
