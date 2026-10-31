-- +goose Up
-- Explicit recipe calculation: 70 g USDA SR Legacy 20080 whole-wheat flour
-- (340 kcal, 13.21 protein, 72.57 carbohydrate, 2.5 fat, 10.7 fibre per 100 g)
-- plus water to an estimated final cooked weight of 100 g. No oil.
UPDATE foods SET calories=238,protein=9.247,carbs=50.799,fat=1.75,fibre=7.49,
 source='Approximate recipe: 70 g whole-wheat flour (USDA SR Legacy 20080) plus water to an estimated 100 g cooked weight, no oil. Flour basis: 340 kcal, 13.21 g protein, 72.57 g carbohydrate, 2.5 g fat, 10.7 g fibre per 100 g. Actual cooked water retention varies.',
 approximate=true,version=version+1 WHERE owner_id IS NULL AND name='Roti, whole wheat, plain';
-- +goose Down
-- Preserve the documented recipe and all historical snapshots.
SELECT 1;
