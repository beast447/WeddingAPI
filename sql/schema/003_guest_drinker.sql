-- +goose Up
ALTER TABLE guests ADD COLUMN Drinker BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE guests DROP COLUMN Drinker;
