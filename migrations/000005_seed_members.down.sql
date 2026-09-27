-- Borra la semilla solo si ningún movimiento la referencia todavía. Así un `down`
-- de un solo paso sobre una base con datos no rompe la FK; en un `down -all` sobre
-- base limpia (el objetivo de CI) no hay entradas y la semilla se borra sin drama.
DELETE FROM members m
WHERE m.id IN (
    '11111111-1111-4111-8111-111111111111',
    '22222222-2222-4222-8222-222222222222'
)
AND NOT EXISTS (
    SELECT 1 FROM point_entries e
    WHERE e.awarded_to = m.id OR e.awarded_by = m.id
);
