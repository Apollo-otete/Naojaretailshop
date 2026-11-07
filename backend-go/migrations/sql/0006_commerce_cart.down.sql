-- 0006 down: undo the cart tables.

DROP TABLE IF EXISTS commerce.cart_items CASCADE;
DROP TABLE IF EXISTS commerce.carts CASCADE;

