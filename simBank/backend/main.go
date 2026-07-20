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

// Processes financial movements - using strict database transactions
func handleTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var txReq IncomingTransaction
	if err := json.NewDecoder(r.Body).Decode(&txReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Malformed JSON request payload."})
		return
	}

	// Basic validation
	if txReq.AccNumber == "" || txReq.Amount <= 0 || txReq.Pin == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid payload. Account, positive amount, and PIN are required."})
		return
	}

	ctx := context.Background()

	// 1. Begin Database Transaction
	tx, err := dbPrimary.Begin(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to initialize secure transaction block."})
		return
	}
	// Defer a rollback; it does nothing if the tx is already committed
	defer tx.Rollback(ctx)

	// 2. Fetch and lock the primary source account row to prevent race conditions (FOR UPDATE)
	var accountID, userID int
	var currentBalance int64
	var dbHashedPin string
	err = tx.QueryRow(ctx,
		"SELECT id, user_id, balance, pin FROM accounts WHERE acc_number = $1 FOR UPDATE",
		txReq.AccNumber,
	).Scan(&accountID, &userID, &currentBalance, &dbHashedPin)

	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Source account not found."})
		return
	}

	// 3. Verify the transaction security PIN
	pinMatch, err := verifyArgon2Match(txReq.Pin, dbHashedPin)
	if err != nil || !pinMatch {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Security validation failed: Invalid PIN."})
		return
	}

	// Generate unique tracking reference string
	rSource := mathrand.NewSource(time.Now().UnixNano())
	rGen := mathrand.New(rSource)
	txReference := fmt.Sprintf("TXN-%d%d", time.Now().Unix(), rGen.Intn(90000)+10000)

	// 4. Process individual financial categories
	switch txReq.Category {
	case "deposit":
		newBalance := currentBalance + txReq.Amount
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", newBalance, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to credit account balance."})
			return
		}

		// Log into transactions table
		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
			VALUES ($1, 'Deposit', $2, 'External/Cash', $3, 'Completed', $4, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReference)

	case "withdraw":
		if currentBalance < txReq.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Transaction rejected: Insufficient funds."})
			return
		}

		newBalance := currentBalance - txReq.Amount
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", newBalance, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to debit account balance."})
			return
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
			VALUES ($1, 'Withdrawal', $2, $3, 'ATM/Branch', 'Completed', $4, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReference)

	case "transfer":
		if txReq.Recipient == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Recipient account number required for transfers."})
			return
		}
		if currentBalance < txReq.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Transaction rejected: Insufficient funds."})
			return
		}

		// Fetch and lock recipient account row
		var destAccountID, destUserID int
		var destBalance int64
		err = tx.QueryRow(ctx, "SELECT id, user_id, balance FROM accounts WHERE acc_number = $1 FOR UPDATE", txReq.Recipient).Scan(&destAccountID, &destUserID, &destBalance)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Recipient account not found."})
			return
		}

		// Debit sender, Credit receiver
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", currentBalance-txReq.Amount, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed processing sender balance update."})
			return
		}
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", destBalance+txReq.Amount, destAccountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed processing recipient balance update."})
			return
		}

		// Log transfer record
		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
			VALUES ($1, 'Transfer', $2, $3, $4, 'Completed', $5, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReq.Recipient, txReference)

		// Create a dynamic notification for recipient asynchronously later or register during tx
		_, _ = tx.Exec(ctx, "INSERT INTO notifications (user_id, message, is_read, created_at) VALUES ($1, $2, FALSE, NOW())",
			destUserID, fmt.Sprintf("You received a transfer of %d from account %s. Ref: %s", txReq.Amount, txReq.AccNumber, txReference))

	case "pay_bill":
		if txReq.Provider == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Bill payment provider is required."})
			return
		}

		// Look up provider by name (no PIN needed)
		var providerID int
		var providerAccNum string
		err = tx.QueryRow(ctx,
			"SELECT id, account_number FROM providers WHERE name = $1 AND is_active = TRUE",
			txReq.Provider).Scan(&providerID, &providerAccNum)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Provider not found or inactive."})
			return
		}

		if currentBalance < txReq.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient funds for bill payment."})
			return
		}

		// Debit account
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", currentBalance-txReq.Amount, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed processing bill payment debit."})
			return
		}

		// Record in bills table (references provider)
		_, err = tx.Exec(ctx, `
			INSERT INTO bills (account_id, provider_id, amount, status, created_at)
			VALUES ($1, $2, $3, 'Paid', NOW())`,
			accountID, providerID, txReq.Amount)

		// Also log in transactions
		_, err = tx.Exec(ctx, `
			INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
			VALUES ($1, 'Bill Payment', $2, $3, $4, 'Completed', $5, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReq.Provider, txReference)
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unsupported transaction category requested."})
		return
	}

	// Double-check pipeline runtime error before final commit
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error finishing transaction processing."})
		return
	}

	// 5. Append system notification to active user context
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (user_id, message, is_read, created_at) 
		VALUES ($1, $2, FALSE, NOW())`,
		userID, fmt.Sprintf("Transaction '%s' of %d completed successfully. Ref: %s", txReq.Category, txReq.Amount, txReference),
	)

	// 6. Commit transaction cleanly to disk
	if err := tx.Commit(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to safely finalize database state changes."})
		return
	}

	// Fetch updated final balance to return to interface
	var updatedBalance int64
	_ = dbPrimary.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", accountID).Scan(&updatedBalance)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Financial transaction processed successfully.",
		"reference": txReference,
		"balance":   updatedBalance,
	})
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
