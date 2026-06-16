-- name: CreateGuest :one
INSERT INTO guests(ID, RsvpID, Name, IsChild)
VALUES($1, $2, $3, $4)
RETURNING *;

-- name: GetGuests :many
SELECT *
FROM guests
ORDER BY RsvpID, Name;
