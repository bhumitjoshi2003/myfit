# Expanded food catalogue — local implementation only

Migration `006_expanded_food_catalogue.sql` adds **197 shared foods**: **112 single-food/label references** and **85 prepared-dish recipe estimates**. On the original 22-food seed this produces **219 shared foods**, with 591 additional serving rows. Existing shared names can reduce the number inserted. No production database, live application, VPS or Nginx was accessed for this work.

## Review and sources

- [review.csv](review.csv): all new names, categories, five nutrients and estimated serving weights.
- [foods.json](foods.json): direct-reference mappings and full recipes, including ingredient masses, preparation notes and estimated final yields.
- [references.json](references.json): pinned nutrient facts, original source descriptions, row identifiers, units/conventions and source archive SHA-256 checksums.
- [additive migration](../migrations/006_expanded_food_catalogue.sql): generated, self-contained SQL; no runtime downloads or new dependencies.

The reference sources are [USDA ARS Standard Reference 28, slightly revised May 2016](https://www.ars.usda.gov/northeast-area/beltsville-md-bhnrc/beltsville-human-nutrition-research-center/methods-and-application-of-food-composition-laboratory/mafcl-site-pages/sr11-sr28/) (a documented historical USDA release, not a claim of current FoodData Central values), [ICMR-NIN IFCT 2017](https://www.nin.res.in/ebooks/IFCT2017.pdf) and [Hilife's manufacturer catalogue](https://hilifefoods.com.np/HilifeProductCatalog.pdf), PDF p58, for that brand's dry soy chunks. Sources were retrieved November 1, 2026. IFCT grain and dehulled-pulse rows were checked visually on PDF pp41–42; paneer follows the previously reviewed L003 reference. USDA's archive includes `sr28_doc.pdf`, documenting the abbreviated file's columns.

No missing fibre values were silently filled for new single-food entries. For example, a USDA jamun record without fibre was excluded. Paneer used as a recipe ingredient retains the existing IFCT convention (blank/below detection fibre treated as zero), disclosed in its source note. Soy chunks are explicitly a **Hilife label reference**, not an unqualified Nutrela or generic-soybean substitution. Product formulations and labels can change; users should use a private label-based copy when appropriate.

## Units and uncertainty

All values are per **100 g edible portion**, including drinks: energy in **kcal**, protein/carbohydrate/fat/fibre in **g**. There is **no implicit g-to-ml conversion**. A "glass, weighed" is an estimated mass serving, not a promise that 250 ml weighs 250 g. Whole-fruit, fish and chicken servings exclude the inedible portions specified in their names. Each new entry includes `Grams` (1 g), `100 grams` (100 g), and a practical estimated portion, such as one idli (40 g), dosa (75 g), paratha (140–150 g), bowl, cup or tablespoon.

USDA carbohydrate is total/by-difference; IFCT reports available carbohydrate excluding fibre. For **new** IFCT-based entries/ingredients, displayed carbohydrate is explicitly calculated as available carbohydrate + dietary fibre to approximate a consistent total-carbohydrate convention. Original IFCT available-carbohydrate facts and energy in kJ are retained in the reference JSON; kcal is kJ / 4.184. This is an estimate across differing analytical methods, not an exact harmonization. Existing catalogue values are untouched.

The 85 prepared dishes are **documented MyFit standard recipe models**, not laboratory measurements or externally validated restaurant recipes. Every model specifies ingredient masses and an assumed edible cooked yield. For each nutrient:

```text
per 100 g dish = sum(ingredient grams × ingredient nutrient per 100 g / 100)
                × 100 / estimated final dish grams
portion nutrient = displayed per-100 g nutrient × serving grams × quantity / 100
```

Dish energy is rounded to whole kcal and macros/fibre to 0.1 g. Ingredient figures retain their published precision; calculated reference energy is rounded to 0.1 kcal. All inserted rows have `approximate=true`. Source notes visible in the existing UI include ingredients, retained oil, yield, relevant proxies and reference IDs/URLs. Water loss/absorption, fermentation, frying retention and milling are not measured. Small unlisted spices and salt are omitted; there is no claim of sodium/micronutrient completeness. Users should weigh their own portion and make a private recipe when their preparation differs. Protein shakes specify their ingredients rather than inventing values for an unspecified whey brand.

## Preservation, repeatability and search

The Up migration locks catalogue writers briefly inside Goose's transaction (SELECT remains available), then inserts `owner_id=NULL` rows only when no shared food has the same case-folded, outer-whitespace-trimmed name. Private foods with the same name do not suppress a shared insert. Servings are created **only for newly inserted foods**, so rerunning Up does not duplicate servings or alter pre-existing serving definitions. No existing food, private recipe, user, favourite or log is updated/deleted. This is scoped seed idempotence, not a new global uniqueness constraint or a fuzzy duplicate-merging system.

Down is intentionally non-destructive (`SELECT 1`): foods may already be logged/favourited. Down does not remove them, and re-Up safely skips them. **Do not run Down as a production rollback**: Goose version metadata would change while data remains. Historical log `reference`, `nutrition`, serving and food-name snapshots remain intact.

The existing API/store queries, ownership rules and React UI are unchanged. Names include useful Indian search terms (e.g. poha, sooji/rava, rajma, bhindi, lauki, paneer). The existing mobile search debounce is 180 ms; results remain capped at 100, so users narrow the library with search. There is no new search infrastructure or index overhead for a 219-food catalogue. A local real-query sample measured 20 `dosa` searches in 26 ms total; this is not a production/mobile latency guarantee.

## Local checks

```sh
python3 database/catalogue/generate.py --check
PYTHONDONTWRITEBYTECODE=1 python3 database/catalogue/test_generate.py
python3 database/catalogue/test-local.py
```

The last command requires the existing **local** `myfit-postgres` PostgreSQL 17 container at 127.0.0.1:5433. It refuses a non-Unix Docker context, creates an unpredictable `myfit_catalogue_test_*` database, runs only the catalogue integration test and drops only that newly created database. It never reads production env files or migrates the real local `myfit` database. No new container/service is created. PostgreSQL SQL features used are standard and require no Neon extension. Actual Neon execution remains unapproved/unperformed.

Coverage: upgrade from Goose v5; 219 shared rows; case/whitespace duplicate handling; same-name private/shared separation; unchanged existing food/serving/user/favourite/log rows; direct Up replay; safe Down/re-Up; shared visibility and private isolation; representative Indian search terms; independent source anchors/recipe arithmetic; all-five-nutrient idli portion scaling; log persistence; unchanged snapshots after a later food edit; and read-only application startup at schema v6. Existing banana fixture searches were made specific to `Banana, raw`, since a larger library legitimately returns banana shakes too. The existing startup compatibility test now expects schema v6.

## Later deployment requires separate approval

This migration changes Goose's expected release version to **6** without changing table structure or grants. The application checks an **exact** migration version at startup. A v5 image cannot restart against a v6 database, even though the new data is structurally compatible. A later approved rollout must therefore coordinate a new image **including migration 006**, the owner-role migration, a restore point and a compatible rollback/forward-fix. Runtime remains `myfit_runtime`; no new objects or extra privileges are required. Do not automatically apply 006 to the live database or reuse the old immutable v5 image as a schema-v6 release.

The historical `backend/cmd/neonvalidate` staging/security tool is intentionally pinned to the original five-migration/22-food baseline; its old reports and guarded workflow are not a v6 rollout procedure. Do not bypass its safety checks or run its owner-role phases against live Neon. Use the separately reviewed production migration/deployment process only after explicit authorization. No publishing, deployment, production migration, DNS change or live verification is part of this implementation.
