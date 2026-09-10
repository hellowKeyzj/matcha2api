WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY package_id, COALESCE(version_id, 0), kind, unit
               ORDER BY starts_at DESC, id DESC
           ) AS rn
    FROM matcha_package_billing_policies
    WHERE active = TRUE
)
UPDATE matcha_package_billing_policies p
SET active = FALSE,
    updated_at = NOW()
FROM ranked r
WHERE p.id = r.id
  AND r.rn > 1;

DROP INDEX IF EXISTS idx_matcha_package_billing_policies_active;

CREATE UNIQUE INDEX IF NOT EXISTS idx_matcha_package_billing_policies_active_unique
    ON matcha_package_billing_policies (package_id, COALESCE(version_id, 0), kind, unit)
    WHERE active = TRUE;
