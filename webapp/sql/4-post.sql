INSERT IGNORE INTO chair_totals (chair_id, total_distance, total_distance_updated_at, latitude, longitude)
SELECT t.chair_id, t.total_distance, t.updated_at, l.latitude, l.longitude
FROM (
  SELECT chair_id, SUM(IFNULL(distance, 0)) AS total_distance, MAX(created_at) AS updated_at
  FROM (SELECT chair_id, created_at,
               ABS(latitude - LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)) +
               ABS(longitude - LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at)) AS distance
        FROM chair_locations) tmp
  GROUP BY chair_id
) t
JOIN chair_locations l ON l.chair_id = t.chair_id AND l.created_at = t.updated_at;
