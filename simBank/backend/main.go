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

/* =============== MAIN FUNCTION ================ */
func main() {
	os.MkdirAll("data", os.ModePerm)

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

	// Routing Setup
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

	// Make sure the email isn't already taken
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

	hashedPassword, err := hashWithArgon2(incomingData.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encrypt security credentials."})
		return
	}

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

	// Generate a quick random account number
	rSource := mathrand.NewSource(time.Now().UnixNano())
	rGen := mathrand.New(rSource)
	generatedAccNum := fmt.Sprintf("100%07d", rGen.Intn(10000000))

	fullName := incomingData.FirstName + " " + incomingData.LastName
	w.WriteHeader(http.StatusOK)

	// Send account number back - frontend needs this to hit the setup-pin route next
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        fmt.Sprintf("Registration phase 1 complete. Account %s pending activation.", generatedAccNum),
		"name":           fullName,
		"account_number": generatedAccNum,
		"balance":        0,
	})
}

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

	// Grab the most recently registered user who doesn't have an active account link yet
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

	hashedPin, err := hashWithArgon2(payload.Pin)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to secure PIN credentials."})
		return
	}

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

	passwordMatch, err := verifyArgon2Match(loginAttempt.Password, hashedPassword)
	if err != nil || !passwordMatch {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password credentials."})
		return
	}

	var accNumber string
	var balance int64
	err = dbPrimary.QueryRow(ctx,
		"SELECT acc_number, balance FROM accounts WHERE user_id = $1 LIMIT 1",
		userID,
	).Scan(&accNumber, &balance)

	if err != nil {
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

	if txReq.AccNumber == "" || txReq.Amount <= 0 || txReq.Pin == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid payload. Account, positive amount, and PIN are required."})
		return
	}

	ctx := context.Background()

	// Start SQL transaction block
	tx, err := dbPrimary.Begin(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to initialize secure transaction block."})
		return
	}
	// Important: automatic safety net. Does nothing if we call tx.Commit() earlier.
	defer tx.Rollback(ctx)

	// FOR UPDATE locks this row so two concurrent requests can't modify the same balance at once
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

	pinMatch, err := verifyArgon2Match(txReq.Pin, dbHashedPin)
	if err != nil || !pinMatch {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Security validation failed: Invalid PIN."})
		return
	}

	rSource := mathrand.NewSource(time.Now().UnixNano())
	rGen := mathrand.New(rSource)
	txReference := fmt.Sprintf("TXN-%d%d", time.Now().Unix(), rGen.Intn(90000)+10000)

	switch txReq.Category {
	case "deposit":
		newBalance := currentBalance + txReq.Amount
		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", newBalance, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to credit account balance."})
			return
		}

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

		// Lock the recipient's row too before changing balances
		var destAccountID, destUserID int
		var destBalance int64
		err = tx.QueryRow(ctx, "SELECT id, user_id, balance FROM accounts WHERE acc_number = $1 FOR UPDATE", txReq.Recipient).Scan(&destAccountID, &destUserID, &destBalance)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Recipient account not found."})
			return
		}

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

		_, err = tx.Exec(ctx, `
            INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
            VALUES ($1, 'Transfer', $2, $3, $4, 'Completed', $5, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReq.Recipient, txReference)

		_, _ = tx.Exec(ctx, "INSERT INTO notifications (user_id, message, is_read, created_at) VALUES ($1, $2, FALSE, NOW())",
			destUserID, fmt.Sprintf("You received a transfer of %d from account %s. Ref: %s", txReq.Amount, txReq.AccNumber, txReference))

	case "pay_bill":
		if txReq.Provider == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Bill payment provider is required."})
			return
		}

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

		_, err = tx.Exec(ctx, "UPDATE accounts SET balance = $1 WHERE id = $2", currentBalance-txReq.Amount, accountID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed processing bill payment debit."})
			return
		}

		_, err = tx.Exec(ctx, `
            INSERT INTO bills (account_id, provider_id, amount, status, created_at)
            VALUES ($1, $2, $3, 'Paid', NOW())`,
			accountID, providerID, txReq.Amount)

		_, err = tx.Exec(ctx, `
            INSERT INTO transactions (account_id, transaction_type, amount, sender, recipient, status, reference, created_at)
            VALUES ($1, 'Bill Payment', $2, $3, $4, 'Completed', $5, NOW())`,
			accountID, txReq.Amount, txReq.AccNumber, txReq.Provider, txReference)
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unsupported transaction category requested."})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error finishing transaction processing."})
		return
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO notifications (user_id, message, is_read, created_at) 
        VALUES ($1, $2, FALSE, NOW())`,
		userID, fmt.Sprintf("Transaction '%s' of %d completed successfully. Ref: %s", txReq.Category, txReq.Amount, txReference),
	)

	// Save everything to disk for real
	if err := tx.Commit(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to safely finalize database state changes."})
		return
	}

	var updatedBalance int64
	_ = dbPrimary.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", accountID).Scan(&updatedBalance)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Financial transaction processed successfully.",
		"reference": txReference,
		"balance":   updatedBalance,
	})
}

func handleTransactionLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	accNumber := r.URL.Query().Get("account_number")
	if accNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account number query parameter required."})
		return
	}

	ctx := context.Background()

	query := `
        SELECT t.created_at, t.reference, t.transaction_type, t.amount, t.status 
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE a.acc_number = $1
        ORDER BY t.created_at DESC`

	rows, err := dbPrimary.Query(ctx, query, accNumber)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to query logs database."})
		return
	}
	defer rows.Close()

	logs := make([]UnifiedLog, 0)
	for rows.Next() {
		var l UnifiedLog
		err := rows.Scan(&l.CreatedTime, &l.Reference, &l.Type, &l.Amount, &l.Status)
		if err != nil {
			continue
		}
		logs = append(logs, l)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(logs)
}

func handleNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	accNumber := r.URL.Query().Get("account_number")
	if accNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Account number required."})
		return
	}

	ctx := context.Background()
	query := `
        SELECT n.id, n.user_id, n.message, n.is_read, n.created_at 
        FROM notifications n
        JOIN accounts a ON n.user_id = a.user_id
        WHERE a.acc_number = $1
        ORDER BY n.created_at DESC`

	rows, err := dbPrimary.Query(ctx, query, accNumber)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to query notifications database."})
		return
	}
	defer rows.Close()

	notifications := make([]Notifications, 0)
	for rows.Next() {
		var n Notifications
		err := rows.Scan(&n.ID, &n.UserID, &n.Message, &n.IsRead, &n.CreatedAt)
		if err != nil {
			continue
		}
		notifications = append(notifications, n)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(notifications)
}

func handleMarkNotificationsAsRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	type ReadPayload struct {
		AccountNumber string `json:"account_number"`
	}

	var payload ReadPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.AccountNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid payload parameters."})
		return
	}

	ctx := context.Background()
	query := `
        UPDATE notifications 
        SET is_read = TRUE 
        WHERE user_id = (SELECT user_id FROM accounts WHERE acc_number = $1 LIMIT 1)`

	_, err := dbPrimary.Exec(ctx, query, payload.AccountNumber)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update database records."})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Notifications marked as read"})
}

/* ======== HELPER FUNCTIONS ======= */
func hashWithArgon2(plainText string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	timeParams := uint32(1)
	memory := uint32(64 * 1024) // 64 MB
	threads := uint8(4)
	keyLength := uint32(32)

	hashedBytes := argon2.IDKey([]byte(plainText), salt, timeParams, memory, threads, keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hashedBytes)

	// Combine everything into a standard format string to save in the DB string field
	encodedHash := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, timeParams, threads, b64Salt, b64Hash)

	return encodedHash, nil
}

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

	// ConstantTimeCompare prevents timing attacks where hackers measure CPU response times
	if subtle.ConstantTimeCompare(existingHash, computedHash) == 1 {
		return true, nil
	}

	return false, nil
}

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
