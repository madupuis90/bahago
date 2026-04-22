-- +goose Up
-- Single authoritative definition of "available units" for a kingdom.
-- A unit is available if it is not currently committed to any campaign,
-- regardless of the campaign's status (en_route, active, or returning).
-- All queries that need available unit counts reference this view so the
-- logic never needs to be duplicated.
CREATE VIEW kingdom_available_units AS
SELECT
    ku.kingdom_id,
    ku.unit_type,
    (ku.count - COALESCE(SUM(kc.count), 0))::int AS count
FROM kingdom_units ku
LEFT JOIN kingdom_campaigns kc
    ON kc.kingdom_id = ku.kingdom_id
    AND kc.unit_type = ku.unit_type
GROUP BY ku.kingdom_id, ku.unit_type;

-- +goose Down
DROP VIEW IF EXISTS kingdom_available_units;
