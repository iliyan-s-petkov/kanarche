-- +goose Up

-- The Sofia slugs were spelled sofiya. area.slug is referenced by area_sensor
-- without ON UPDATE CASCADE, so each row is copied under the new slug, its
-- sensor links are moved, and the old row is deleted. Without this the
-- deploy-time boundaries import would add a second, overlapping Sofia beside
-- the stale one. A database that never held the old rows is untouched.
INSERT INTO area (slug, kind, name_bg, name_en, geom, country_code)
SELECT 'sofia', kind, name_bg, name_en, geom, country_code FROM area WHERE slug = 'sofiya'
ON CONFLICT (slug) DO NOTHING;
INSERT INTO area (slug, kind, name_bg, name_en, geom, country_code)
SELECT 'sofia-grad-oblast', kind, name_bg, name_en, geom, country_code FROM area WHERE slug = 'sofiya-grad-oblast'
ON CONFLICT (slug) DO NOTHING;

INSERT INTO area_sensor (area_slug, sensor_id)
SELECT 'sofia', sensor_id FROM area_sensor WHERE area_slug = 'sofiya'
ON CONFLICT DO NOTHING;
INSERT INTO area_sensor (area_slug, sensor_id)
SELECT 'sofia-grad-oblast', sensor_id FROM area_sensor WHERE area_slug = 'sofiya-grad-oblast'
ON CONFLICT DO NOTHING;

DELETE FROM area WHERE slug IN ('sofiya', 'sofiya-grad-oblast');

-- +goose Down
-- The rename is not reversed: the old spelling was a typo.
SELECT 1;
