-- +goose Up
-- IFCT 2017, table 1, printed page 19, L003. Energy is published in kJ.
-- 1278 kJ / 4.184 = 305.4493307839 kcal. This changes catalogue values only.
UPDATE foods SET calories=1278.0/4.184,protein=18.86,carbs=2.41,fat=24.78,fibre=0,
 source='ICMR-NIN Indian Food Composition Tables 2017, L003, table 1 p.19; 1278 kJ converted to kcal. Available carbohydrate; fibre not reported (displayed as 0). Milk fat and moisture vary; prefer your label. https://www.nin.res.in/ebooks/IFCT2017.pdf',
 approximate=true,version=version+1 WHERE owner_id IS NULL AND name='Paneer, plain';
-- +goose Down
-- Keep the corrected reference; historical log snapshots are never rewritten.
SELECT 1;
