package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/beast447/WeddingAPI/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type state struct {
	queries *database.Queries
}

type RSVP struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Attending bool   `json:"attending"`
	Allergies string `json:"allergies,omitempty"`
	Drinker   bool   `json:"drinker,omitempty"`
	Questions string `json:"questions,omitempty"`
}

func main() {
	godotenv.Load(".env")
	server := gin.Default()
	db, err := pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	dbQueries := database.New(db)

	programState := state{
		queries: dbQueries,
	}

	server.POST("/api/rsvps", func(c *gin.Context) {
		var rsvp RSVP
		decoder := json.NewDecoder(c.Request.Body)
		if err := decoder.Decode(&rsvp); err != nil {
			c.AbortWithError(500, err)
			return
		}

		user, err := programState.queries.CreateRsvp(c.Request.Context(), database.CreateRsvpParams{
			ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Createdat: pgtype.Timestamp{Time: time.Now(), Valid: true},
			Name:      rsvp.Name,
			Email:     rsvp.Email,
			Attending: rsvp.Attending,
			Allergies: pgtype.Text{String: rsvp.Allergies, Valid: true},
			Drinker:   rsvp.Drinker,
			Question:  pgtype.Text{String: rsvp.Questions, Valid: true},
		})

		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, user.Name)
	})

	server.GET("/api/rsvps", func(c *gin.Context) {
		users, err := programState.queries.GetRsvps(c.Request.Context())
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, users)
	})

	server.Run()
}
