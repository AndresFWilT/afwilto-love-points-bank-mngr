-- Restaura la vista de la migración 000004 (sin canjes).
DROP VIEW member_balances;
CREATE VIEW member_balances AS
SELECT
    m.id                                        AS member_id,
    m.display_name,
    COALESCE(SUM(e.amount) FILTER (
        WHERE e.status = 'active'), 0)::int      AS points_earned,
    COUNT(e.id) FILTER (WHERE e.status = 'active')::int AS entries_count,
    MAX(e.occurred_at) FILTER (
        WHERE e.status = 'active')               AS last_entry_at
FROM members m
LEFT JOIN point_entries e ON e.awarded_to = m.id
GROUP BY m.id, m.display_name;
