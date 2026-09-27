DROP VIEW member_balances;
CREATE VIEW member_balances AS
SELECT
    m.id                                                            AS member_id,
    m.display_name,
    COALESCE(SUM(e.amount) FILTER (WHERE e.status = 'active'), 0)::int AS points_earned,
    COUNT(e.id) FILTER (WHERE e.status = 'active')::int             AS entries_count,
    MAX(e.occurred_at) FILTER (WHERE e.status = 'active')          AS last_entry_at,
    COALESCE(r.points_redeemed, 0)::int                            AS points_redeemed,
    (COALESCE(SUM(e.amount) FILTER (WHERE e.status = 'active'), 0)
        - COALESCE(r.points_redeemed, 0))::int                     AS points_available
FROM members m
LEFT JOIN point_entries e ON e.awarded_to = m.id
LEFT JOIN (
    SELECT member_id, SUM(cost_at_redemption)::int AS points_redeemed
    FROM redemptions WHERE status = 'active'
    GROUP BY member_id
) r ON r.member_id = m.id
GROUP BY m.id, m.display_name, r.points_redeemed;
