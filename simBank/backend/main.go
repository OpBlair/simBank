package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	mathrand "math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

/* ============ DATA STRUCTS =============== */

type UserData struct {
	ID        int       `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
}

type AccountsData struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	AccNumber   string    `json:"acc_number"`
	Balance     int64     `json:"balance"`
	AccountType string    `json:"acc_type"`
	Status      string    `json:"status"`
	Pin         string    `json:"pin"`
	CreatedAt   time.Time `json:"created_at"`
}

type Transactions struct {
	ID              int       `json:"id"`
	AccountID       int       `json:"account_id"`
	TransactionType string    `json:"transaction_type"`
	Amount          int64     `json:"amount"`
	Sender          string    `json:"sender"`
	Recipient       string    `json:"recipient"`
	Status          string    `json:"status"`
	Reference       string    `json:"reference"`
	CreatedAt       time.Time `json:"created_at"`
}

type Bills struct {
	ID        int       `json:"id"`
	AccountID int       `json:"account_id"`
	Provider  string    `json:"provider"`
	Amount    int64     `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Notifications struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type UnifiedLog struct {
	CreatedTime time.Time `json:"timestamp"`
	Reference   string    `json:"reference"`
	Type        string    `json:"type"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
}

type IncomingTransaction struct {
	Category  string `json:"category"`
	Type      string `json:"type"`
	Amount    int64  `json:"amount"`
	Provider  string `json:"provider"`
	Pin       string `json:"pin"`
	AccNumber string `json:"account_number"`
	Recipient string `json:"recipient"`
}

/* === DATABASE CONNECTION POOLS === */
var dbPrimary *pgxpool.Pool

/* dummy data to prevent compiler from complaining about unused packages */
var _ = argon2.IDKey
var _ = context.Background

/* =============== MAIN FUNCTION ================ */

func main() {
	os.MkdirAll("data", os.ModePerm)
	// Seed your random generator for unique account numbers
	//mathRand.Seed(time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connStr := "postgres://user_name:password@host_name:port_number/database_name?sslmode=disable"

	var err error
	dbPrimary, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Unable to connect to Database: %v\n", err)
		os.Exit(1)
	}
	defer dbPrimary.Close()

	if err = dbPrimary.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Database server unreachable: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Connected to PostgreSQL successfully!")

	var databaseName string
	err = dbPrimary.QueryRow(
		context.Background(),
		"SELECT current_database();",
	).Scan(&databaseName)

	if err != nil {
		log.Fatal("Query failed:", err)
	}

	fmt.Println(" Connected database:", databaseName)

	/* ===== API ROUTES ==== */
	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/setup-pin", handleSetupPin)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/transaction", handleTransaction)
	http.HandleFunc("/api/transactionLogs", handleTransactionLogs)
	http.HandleFunc("/api/notifications", handleNotifications)
	http.HandleFunc("/api/notifications/read", handleMarkNotificationsAsRead)

	fmt.Println("Server starting on http://localhost:8080...")
	http.ListenAndServe(":8080", nil)
}

/* ========= HANDLERS ========== */

// Handles user registration - generates account number, returns it, prompts for PIN
func handleRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var incomingData UserData
	if err := json.NewDecoder(r.Body).Decode(&incomingData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Malformed JSON request payload."}`))
		return
	}

	if incomingData.FirstName == "" || incomingData.LastName == "" || incomingData.Email == "" || incomingData.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Registration rejected: All fields are strictly required."}`))
		return
	}

	ctx := context.Background()

	// 1. Check if user already exists
	var exists bool
	err := dbPrimary.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))", strings.TrimSpace(incomingData.Email)).Scan(&exists)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database validation failed."})
		return
	}
	if exists {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": "An account with this email already exists."})
		return
	}

	// 2. Hash Password using Argon2 with proper salt
	hashedPassword, err := hashWithArgon2(incomingData.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encrypt security credentials."})
		return
	}

	// 3. Save User row into PostgreSQL
	var newUserID int
	err = dbPrimary.QueryRow(ctx,
		"INSERT INTO users (first_name, last_name, email, password, created_at) VALUES ($1, $2, $3, $4, NOW()) RETURNING id",
		incomingData.FirstName, incomingData.LastName, strings.TrimSpace(incomingData.Email), hashedPassword,
	).Scan(&newUserID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save user record."})
		return
	}

	// 4. Generate account number
	rSource := mathrand.NewSource(time.Now().UnixNano())
	rGen := mathrand.New(rSource)
	generatedAccNum := fmt.Sprintf("100%07d", rGen.Intn(10000000))

	fullName := incomingData.FirstName + " " + incomingData.LastName
	w.WriteHeader(http.StatusOK)

	// Send account number back - frontend will pass it to setup-pin
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        fmt.Sprintf("Registration phase 1 complete. Account %s pending activation.", generatedAccNum),
		"name":           fullName,
		"account_number": generatedAccNum,
		"balance":        0,
	})
}

// Handles PIN Setup - creates account in DB with hashed PIN
func handleSetupPin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	type PinPayload struct {
		AccNumber string `json:"account_number"`
		Pin       string `json:"pin"`
	}

	var payload PinPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Malformed request payload."})
		return
	}

	if payload.AccNumber == "" || len(payload.Pin) != 4 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "A valid account number and 4-digit PIN are required."})
		return
	}

	ctx := context.Background()

	// 1. Get the latest registered user who doesn't have an account yet
	var userID int
	err := dbPrimary.QueryRow(ctx, `
		SELECT u.id FROM users u 
		LEFT JOIN accounts a ON u.id = a.user_id 
		WHERE a.id IS NULL 
		ORDER BY u.created_at DESC LIMIT 1`,
	).Scan(&userID)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No matching user found for this account initialization."})
		return
	}

	// 2. Hash the PIN using Argon2 with proper salt
	hashedPin, err := hashWithArgon2(payload.Pin)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to secure PIN credentials."})
		return
	}

	// 3. Insert account into DB with hashed PIN - NOW account is created
	_, err = dbPrimary.Exec(ctx, `
		INSERT INTO accounts (user_id, acc_number, balance, acc_type, status, pin, created_at) 
		VALUES ($1, $2, 0, 'Checking', 'Active', $3, NOW())`,
		userID, payload.AccNumber, hashedPin,
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create account profile inside database."})
		return
	}

	createNotification(userID, fmt.Sprintf("Welcome to SimBank! Account %s successfully activated with security PIN.", payload.AccNumber))

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Security PIN configured safely. Account database synchronized successfully!",
	})
}

// Verifies credentials and logs the user in - uses DB with password verification
func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var loginAttempt UserData
	json.NewDecoder(r.Body).Decode(&loginAttempt)

	if loginAttempt.Email == "" || loginAttempt.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Email and password fields cannot be empty."}`))
		return
	}

	ctx := context.Background()

	// 1. Query user by email
	var userID int
	var firstName, lastName, hashedPassword string
	err := dbPrimary.QueryRow(ctx,
		"SELECT id, first_name, last_name, password FROM users WHERE LOWER(email) = LOWER($1)",
		strings.TrimSpace(loginAttempt.Email),
	).Scan(&userID, &firstName, &lastName, &hashedPassword)

	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password credentials."})
		return
	}

	// 2. Verify password against hashed version using Argon2
	passwordMatch, err := verifyArgon2Match(loginAttempt.Password, hashedPassword)
	if err != nil || !passwordMatch {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password credentials."})
		return
	}

	// 3. Get user's account data from DB
	var accNumber string
	var balance int64
	err = dbPrimary.QueryRow(ctx,
		"SELECT acc_number, balance FROM accounts WHERE user_id = $1 LIMIT 1",
		userID,
	).Scan(&accNumber, &balance)

	if err != nil {
		// User has no account yet (registered but didn't setup PIN)
		accNumber = "N/A"
		balance = 0
	}

	fullName := firstName + " " + lastName
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Login verified!",
		"name":           fullName,
		"account_number": accNumber,
		"balance":        balance,
	})
}

// Processes financial movements - kept as is for now
func handleTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Transaction handler - to be updated"})
}

// Transaction logs handler
func handleTransactionLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]UnifiedLog{})
}

// Notifications handler
func handleNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]Notifications{})
}

// Mark notifications as read handler
func handleMarkNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Notifications marked as read"})
}

/* ======== HELPER FUNCTIONS ======= */

// Hash password with Argon2id using cryptographically secure random salt
func hashWithArgon2(plainText string) (string, error) {
	// Generate 16-byte cryptographically secure random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	timeParams := uint32(1)
	memory := uint32(64 * 1024) // 64 MB
	threads := uint8(4)
	keyLength := uint32(32)

	hashedBytes := argon2.IDKey([]byte(plainText), salt, timeParams, memory, threads, keyLength)

	// Encode salt and hash to base64
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hashedBytes)

	// Standard format with embedded parameters
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, timeParams, threads, b64Salt, b64Hash)

	return encodedHash, nil
}

// Verify password against Argon2 hash with constant-time comparison
func verifyArgon2Match(plainText, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid Argon2 hash format")
	}

	var memory, timeParams uint32
	var threads uint8

	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeParams, &threads)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	existingHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	keyLength := uint32(len(existingHash))
	computedHash := argon2.IDKey([]byte(plainText), salt, timeParams, memory, threads, keyLength)

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(existingHash, computedHash) == 1 {
		return true, nil
	}

	return false, nil
}

// Creates notification record in DB
func createNotification(userID int, message string) {
	ctx := context.Background()

	_, err := dbPrimary.Exec(ctx, `
		INSERT INTO notifications (user_id, message, is_read, created_at) 
		VALUES ($1, $2, FALSE, NOW())`,
		userID, message,
	)

	if err != nil {
		log.Printf("Error creating notification: %v", err)
	}
}
