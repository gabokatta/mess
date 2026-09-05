-- A category's colour used to be its position in the list every screen read,
-- so reordering repainted Month and Year and the ninth category silently
-- shared the first's hue. It is a field now.
ALTER TABLE category ADD COLUMN color_index INTEGER NOT NULL DEFAULT 0
    CHECK (color_index BETWEEN 0 AND 7);

-- Backfill each row with the position it already had, so no existing database
-- changes colour on upgrade. Eight is the palette's size.
UPDATE category
SET color_index = (
    SELECT COUNT(*)
    FROM category AS earlier
    WHERE earlier.sort_order < category.sort_order
       OR (earlier.sort_order = category.sort_order AND earlier.name < category.name)
) % 8;
