-- 0005 down: undo the catalogue tables.

DROP TABLE IF EXISTS catalogue.product_images CASCADE;
DROP TABLE IF EXISTS catalogue.products CASCADE;
DROP TABLE IF EXISTS catalogue.categories CASCADE;
