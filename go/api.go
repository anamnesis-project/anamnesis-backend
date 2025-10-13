package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIFunc func(w http.ResponseWriter, r *http.Request) error

type TokenClaim string
const (
	userIdClaim = TokenClaim("userId")
)

func makeHandler(handler APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Request from %s\t%s %s\n", r.RemoteAddr, r.Method, r.URL.Path)

		err := handler(w, r)
		if err != nil {
			if e, ok := err.(APIError); ok {
				fmt.Println("API error:", e.Msg)
				writeJSON(w, e.StatusCode, e)
			} else {
				fmt.Println("error:", err)
				writeJSON(w, http.StatusInternalServerError, "Internal Error")
			}
		}
	}
}

var jwtSecret = []byte("TODO: stop using me")

type CustomClaims struct {
	UserId int `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *Server) jwtMiddleware(handler APIFunc) APIFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var tokenString string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && !strings.HasPrefix(authHeader, "Bearer ") {
			return writeJSON(w, http.StatusUnauthorized, "Missing or invalid Authorization header")
		}
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			return InvalidToken()
		}

		claims, ok := token.Claims.(*CustomClaims)
		if !ok {
			return InvalidToken()
		}

		// contains database query for user permissions
		accessAllowed, err := s.getEmployeeAccess(claims.UserId)
		if err != nil {
			return InvalidToken()
		}

		if !accessAllowed {
			return AccessNotAllowed()
		}

		ctx := context.WithValue(r.Context(), userIdClaim, claims.UserId)
		r = r.WithContext(ctx)

		return handler(w, r)
	}
}

func createJWT(userId int) (string, error) {
	claims := CustomClaims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func getIdFromToken(r *http.Request) (int, error) {
	id, ok := r.Context().Value(userIdClaim).(int)
	if !ok {
		return 0, fmt.Errorf("unable to retrieve id from context")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func getPathId(wildcard string, r *http.Request) (int, error) {
	v := r.PathValue(wildcard)
	if v == "" {
		return 0, errors.New("unable to get path id")
	}

	id, err := strconv.Atoi(v)
	return id, err
}

type Server struct {
	port string
	db   *pgxpool.Pool
}

func NewServer(port string) *Server {
	s := &Server{
		port: port,
	}

	s.initDB()

	http.HandleFunc("GET /reports", makeHandler(s.jwtMiddleware(s.handleGetReports)))
	http.HandleFunc("GET /reports/{id}", makeHandler(s.jwtMiddleware(s.handleGetReportById)))
	http.HandleFunc("GET /reports/{id}/pdf", makeHandler(s.jwtMiddleware(s.handleGetReportPDF)))
	http.HandleFunc("POST /reports", makeHandler(s.handleCreateReport))

	http.HandleFunc("GET /patients", makeHandler(s.jwtMiddleware(s.handleGetPatients)))
	http.HandleFunc("GET /patients/{id}", makeHandler(s.jwtMiddleware(s.handleGetPatientById)))
	http.HandleFunc("GET /patients/{id}/reports", makeHandler(s.jwtMiddleware(s.handleGetPatientReports)))

	http.HandleFunc("GET /employees", makeHandler(s.jwtMiddleware(s.handleGetEmployees)))
	http.HandleFunc("GET /employees/{id}", makeHandler(s.jwtMiddleware(s.handleGetEmployeeById)))
	http.HandleFunc("PATCH /employees/{id}", makeHandler(s.jwtMiddleware(s.handlePatchEmployeePermissions)))

	http.HandleFunc("POST /login", makeHandler(s.handleLogin))
	http.HandleFunc("POST /register", makeHandler(s.handleRegister))

	return s
}

func (s *Server) Run() {
	fmt.Println("Server running on", s.port)
	err := http.ListenAndServe(s.port, nil)
	s.db.Close()
	log.Fatal(err)
}

func (s *Server) initDB() {
	var err error
	s.db, err = pgxpool.New(context.Background(), os.Getenv("DB_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if err = s.db.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}
}
