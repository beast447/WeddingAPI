-- name: CreateGuest :one
INSERT INTO guests(ID, RsvpID, Name, IsChild, Drinker)
VALUES($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGuests :many
SELECT *
FROM guests
ORDER BY RsvpID, Name;
