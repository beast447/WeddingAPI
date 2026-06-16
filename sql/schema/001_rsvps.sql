-- +goose Up
CREATE TABLE rsvps(
    ID UUID PRIMARY KEY,
    CreatedAt TIMESTAMP NOT NULL,
    Name TEXT NOT NULL,
    Email TEXT NOT NULL UNIQUE,
    Attending BOOLEAN NOT NULL,
    Allergies TEXT,
    Drinker BOOLEAN NOT NULL,
    Question TEXT
);

-- +goose Down
DROP TABLE rsvps;