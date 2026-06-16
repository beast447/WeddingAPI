package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/beast447/WeddingAPI/internal/database"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const sessionTTL = 24 * time.Hour

type state struct {
	db            *pgxpool.Pool
	queries       *database.Queries
	adminPassword string
	sessionSecret string
}

// Guest is an additional party member (spouse/child) sent with an RSVP.
type Guest struct {
	Name    string `json:"name"`
	IsChild bool   `json:"isChild"`
}

type RSVP struct {
	Name             string  `json:"name"`
	Email            string  `json:"email"`
	Attending        bool    `json:"attending"`
	Allergies        string  `json:"allergies,omitempty"`
	Drinker          bool    `json:"drinker,omitempty"`
	Questions        string  `json:"questions,omitempty"`
	AdditionalGuests []Guest `json:"additionalGuests,omitempty"`
}

// GuestResponse is the JSON shape for an additional party member.
type GuestResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsChild bool   `json:"isChild"`
}

// RSVPResponse is the JSON shape the frontend consumes (the sqlc-generated
// database.Rsvp has no JSON tags and pgtype fields, so it can't be returned directly).
type RSVPResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Email       string          `json:"email"`
	Attending   bool            `json:"attending"`
	Allergies   string          `json:"allergies"`
	Drinker     bool            `json:"drinker"`
	Questions   string          `json:"questions"`
	SubmittedAt string          `json:"submittedAt"`
	Guests      []GuestResponse `json:"guests"`
}

func toRSVPResponse(r database.Rsvp, guests []GuestResponse) RSVPResponse {
	if guests == nil {
		guests = []GuestResponse{}
	}
	return RSVPResponse{
		ID:          uuid.UUID(r.ID.Bytes).String(),
		Name:        r.Name,
		Email:       r.Email,
		Attending:   r.Attending,
		Allergies:   r.Allergies.String,
		Drinker:     r.Drinker,
		Questions:   r.Question.String,
		SubmittedAt: r.Createdat.Time.Format(time.RFC3339),
		Guests:      guests,
	}
}

// sign returns a hex HMAC-SHA256 of msg keyed by the session secret.
func sign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// generateToken issues a stateless, signed token of the form "<expiry>.<sig>".
func generateToken(secret string, ttl time.Duration) string {
	exp := strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	return exp + "." + sign(secret, exp)
}

// validateToken verifies the signature and that the token has not expired.
func validateToken(secret, token string) bool {
	exp, sig, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	expected := sign(secret, exp)
	if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) != 1 {
		return false
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return false
	}
	return true
}

// requireAuth rejects requests without a valid Bearer token.
func requireAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || !validateToken(secret, token) {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func main() {
	godotenv.Load(".env")

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}

	server := gin.Default()
	server.Use(cors.New(cors.Config{
		AllowOrigins: []string{frontendURL},
		AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
	}))

	db, err := pgxpool.New(context.Background(), mustEnv("DB_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	programState := state{
		db:            db,
		queries:       database.New(db),
		adminPassword: mustEnv("ADMIN_PASSWORD"),
		sessionSecret: mustEnv("SESSION_SECRET"),
	}

	server.POST("/api/auth/login", func(c *gin.Context) {
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			c.JSON(400, gin.H{"error": "invalid request"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(body.Password), []byte(programState.adminPassword)) != 1 {
			c.JSON(401, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(200, gin.H{"token": generateToken(programState.sessionSecret, sessionTTL)})
	})

	server.POST("/api/rsvps", func(c *gin.Context) {
		ctx := c.Request.Context()
		var rsvp RSVP
		decoder := json.NewDecoder(c.Request.Body)
		if err := decoder.Decode(&rsvp); err != nil {
			c.AbortWithError(500, err)
			return
		}

		// Insert the RSVP and its additional guests atomically.
		tx, err := programState.db.Begin(ctx)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		defer tx.Rollback(ctx)
		qtx := programState.queries.WithTx(tx)

		rsvpID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
		user, err := qtx.CreateRsvp(ctx, database.CreateRsvpParams{
			ID:        rsvpID,
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

		for _, g := range rsvp.AdditionalGuests {
			if strings.TrimSpace(g.Name) == "" {
				continue
			}
			if _, err := qtx.CreateGuest(ctx, database.CreateGuestParams{
				ID:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Rsvpid:  rsvpID,
				Name:    g.Name,
				Ischild: g.IsChild,
			}); err != nil {
				c.AbortWithError(500, err)
				return
			}
		}

		if err := tx.Commit(ctx); err != nil {
			c.AbortWithError(500, err)
			return
		}
		c.JSON(200, user.Name)
	})

	server.GET("/api/rsvps", requireAuth(programState.sessionSecret), func(c *gin.Context) {
		ctx := c.Request.Context()
		users, err := programState.queries.GetRsvps(ctx)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		guests, err := programState.queries.GetGuests(ctx)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		// Group guests by their RSVP id.
		guestsByRSVP := make(map[string][]GuestResponse)
		for _, g := range guests {
			rsvpID := uuid.UUID(g.Rsvpid.Bytes).String()
			guestsByRSVP[rsvpID] = append(guestsByRSVP[rsvpID], GuestResponse{
				ID:      uuid.UUID(g.ID.Bytes).String(),
				Name:    g.Name,
				IsChild: g.Ischild,
			})
		}
		resp := make([]RSVPResponse, len(users))
		for i, u := range users {
			resp[i] = toRSVPResponse(u, guestsByRSVP[uuid.UUID(u.ID.Bytes).String()])
		}
		c.JSON(200, resp)
	})

	server.DELETE("/api/rsvps/:id", requireAuth(programState.sessionSecret), func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid id"})
			return
		}
		rows, err := programState.queries.DeleteRsvp(c.Request.Context(), pgtype.UUID{Bytes: id, Valid: true})
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		if rows == 0 {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.Status(204)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server.Run(":" + port)
}
