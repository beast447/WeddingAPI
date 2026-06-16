-- name: CreateRsvp :one
INSERT INTO rsvps(ID, CreatedAt, Name, Email, Attending, Allergies, Drinker, Question) 
VALUES(
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING *;

-- name: GetRsvps :many
SELECT *
FROM rsvps
ORDER BY CreatedAt;

-- name: GetRsvpByEmail :one
SELECT Email
FROM rsvps
WHERE Email = $1;

-- name: DeleteRsvp :execrows
DELETE FROM rsvps
WHERE ID = $1;