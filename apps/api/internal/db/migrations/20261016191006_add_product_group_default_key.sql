-- +goose Up
-- default_key marks the groups the app created itself, so the web can show
-- them translated. Renaming a group clears it.
ALTER TABLE product_groups ADD COLUMN default_key text;

UPDATE product_groups g SET default_key = d.key
FROM (VALUES
    ('Produce', 'produce'),
    ('Meat & fish', 'meat_fish'),
    ('Dairy & eggs', 'dairy_eggs'),
    ('Bakery', 'bakery'),
    ('Pantry', 'pantry'),
    ('Frozen', 'frozen'),
    ('Snacks & sweets', 'snacks_sweets'),
    ('Drinks', 'drinks'),
    ('Alcohol', 'alcohol'),
    ('Cleaning & household', 'cleaning_household'),
    ('Personal care', 'personal_care'),
    ('Other', 'other')
) AS d(name, key)
WHERE g.name = d.name;

CREATE UNIQUE INDEX product_groups_default_key_key ON product_groups (workspace_id, default_key)
    WHERE default_key IS NOT NULL;

-- +goose Down
DROP INDEX product_groups_default_key_key;
ALTER TABLE product_groups DROP COLUMN default_key;
