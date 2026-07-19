package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
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
	ID            int    `json:"id"`
	UserID        int    `json:"user_id"`
	AccountNumber string `json:"acc_number"`
	Balance       int    `json:"balance"`
	AccountType   string `json:"acc_type"`
	Status        string `json:"status"`
	Pin           string `json:"pin"`
}

type Transactions struct {
	ID              int       `json:"id"`
	AccountID       int       `json:"account_id"`
	TransactionType string    `json:"transaction_type"`
	Amount          int       `json:"amount"`
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
	Amount    int       `json:"amount"`
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
	Amount      int       `json:"amount"`
	Status      string    `json:"status"`
}

type IncomingTransaction struct {
	Category      string `json:"category"`
	Type          string `json:"type"`
	Amount        int    `json:"amount"`
	Provider      string `json:"provider"`
	Pin           string `json:"pin"`
	AccountNumber string `json:"account_number"`
	Recipient     string `json:"recipient"`
}

/* =============== MAIN FUNCTION ================ */

func main() {
	os.MkdirAll("data", os.ModePerm)

	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/transaction", handleTransaction)
	http.HandleFunc("/api/transactionLogs", handleTransactionLogs)
	http.HandleFunc("/api/notifications", handleNotifications)

	fmt.Println("Server starting on http://localhost:8080...")
	http.ListenAndServe(":8080", nil)
}

/* ========= HANDLERS ========== */

// Handles user registration and account creation
func handleRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		return
	}

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

	var usersList []UserData
	fileBytes, err := os.ReadFile("data/users.json")
	if err == nil && len(fileBytes) > 0 {
		json.Unmarshal(fileBytes, &usersList)
	}

	for _, user := range usersList {
		if strings.EqualFold(strings.TrimSpace(user.Email), strings.TrimSpace(incomingData.Email)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "An account with this email already exists.",
			})
			return
		}
	}

	nextUserID := 1
	if len(usersList) > 0 {
		nextUserID = usersList[len(usersList)-1].ID + 1
	}

	incomingData.ID = nextUserID
	incomingData.CreatedAt = time.Now()
	usersList = append(usersList, incomingData)

	fileData, _ := json.MarshalIndent(usersList, "", "  ")
	_ = os.WriteFile("data/users.json", fileData, 0644)

	var accountsList []AccountsData
	accBytes, err := os.ReadFile("data/accounts.json")
	if err == nil && len(accBytes) > 0 {
		json.Unmarshal(accBytes, &accountsList)
	}

	nextAccountID := 1
	if len(accountsList) > 0 {
		nextAccountID = accountsList[len(accountsList)-1].ID + 1
	}

	rSource := rand.NewSource(time.Now().UnixNano())
	rGen := rand.New(rSource)
	generatedAccNum := fmt.Sprintf("100%07d", rGen.Intn(10000000))

	newAccount := AccountsData{
		ID:            nextAccountID,
		UserID:        incomingData.ID,
		AccountNumber: generatedAccNum,
		Balance:       0,
		AccountType:   "Checking",
		Status:        "Active",
	}
	accountsList = append(accountsList, newAccount)

	accFileData, _ := json.MarshalIndent(accountsList, "", "  ")
	_ = os.WriteFile("data/accounts.json", accFileData, 0644)

	fullName := incomingData.FirstName + " " + incomingData.LastName
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON := fmt.Sprintf(`{"message": "Registration saved successfully! Account %s opened.", "name": "%s"}`, generatedAccNum, fullName)
	w.Write([]byte(responseJSON))
}

// Verifies credentials and logs the user in
func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}

	var loginAttempt UserData
	json.NewDecoder(r.Body).Decode(&loginAttempt)

	if loginAttempt.Email == "" || loginAttempt.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Email and password fields cannot be empty."}`))
		return
	}

	fileBytes, err := os.ReadFile("data/users.json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "No registered users found. Please register first."}`))
		return
	}

	var usersList []UserData
	json.Unmarshal(fileBytes, &usersList)

	var userFound bool
	var matchedUser UserData

	for _, user := range usersList {
		if user.Email == loginAttempt.Email && user.Password == loginAttempt.Password {
			userFound = true
			matchedUser = user
			break
		}
	}

	if userFound {
		fullName := matchedUser.FirstName + " " + matchedUser.LastName
		accountNumber := "N/A"
		balance := 0

		accBytes, err := os.ReadFile("data/accounts.json")
		if err == nil && len(accBytes) > 0 {
			var accountsList []AccountsData
			json.Unmarshal(accBytes, &accountsList)

			for _, acc := range accountsList {
				if acc.UserID == matchedUser.ID {
					accountNumber = acc.AccountNumber
					balance = acc.Balance
					break
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		responsePayload := map[string]interface{}{
			"message":        "Login verified!",
			"name":           fullName,
			"account_number": accountNumber,
			"balance":        balance,
		}
		json.NewEncoder(w).Encode(responsePayload)

	} else {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid email or password credentials."}`))
	}
}

// Processes financial movements (deposit, withdraw, transfer, bills)
func handleTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var txAttempt IncomingTransaction
	if err := json.NewDecoder(r.Body).Decode(&txAttempt); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Malformed request payload."})
		return
	}

	accBytes, err := os.ReadFile("data/accounts.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Could not access accounts database."})
		return
	}

	var accountsList []AccountsData
	json.Unmarshal(accBytes, &accountsList)

	if len(accountsList) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "No bank accounts found."})
		return
	}

	targetIdx := -1
	for i := 0; i < len(accountsList); i++ {
		if accountsList[i].AccountNumber == txAttempt.AccountNumber {
			targetIdx = i
			break
		}
	}

	if targetIdx == -1 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Target bank account not found."})
		return
	}

	if accountsList[targetIdx].Pin != "" && accountsList[targetIdx].Pin != txAttempt.Pin {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Security Authorization Failed: Invalid PIN."})
		return
	}

	if txAttempt.Category == "bill_pay" {
		if txAttempt.Provider == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Utility provider is required for bill payments."})
			return
		}
		txAttempt.Type = "withdraw"
	}

	recipientIdx := -1

	switch txAttempt.Type {
	case "deposit":
		accountsList[targetIdx].Balance += txAttempt.Amount

	case "withdraw":
		if accountsList[targetIdx].Balance < txAttempt.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient available funds for this operation."})
			return
		}
		accountsList[targetIdx].Balance -= txAttempt.Amount

	case "transfer":
		if accountsList[targetIdx].Balance < txAttempt.Amount {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Insufficient available funds for transfer."})
			return
		}
		if txAttempt.Recipient == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Recipient account number is required for transfers."})
			return
		}
		if txAttempt.Recipient == txAttempt.AccountNumber {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Cannot transfer funds to your own account."})
			return
		}

		for i := 0; i < len(accountsList); i++ {
			if accountsList[i].AccountNumber == txAttempt.Recipient {
				recipientIdx = i
				break
			}
		}

		if recipientIdx == -1 {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Recipient account number not found."})
			return
		}

		accountsList[targetIdx].Balance -= txAttempt.Amount
		accountsList[recipientIdx].Balance += txAttempt.Amount

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unsupported transaction type."})
		return
	}

	updatedAccBytes, _ := json.MarshalIndent(accountsList, "", "  ")
	_ = os.WriteFile("data/accounts.json", updatedAccBytes, 0644)

	if txAttempt.Category == "bill_pay" {
		var billsList []Bills
		billBytes, err := os.ReadFile("data/bills.json")
		if err == nil && len(billBytes) > 0 {
			json.Unmarshal(billBytes, &billsList)
		}

		nextBillID := 1
		if len(billsList) > 0 {
			nextBillID = billsList[len(billsList)-1].ID + 1
		}

		newBillRecord := Bills{
			ID:        nextBillID,
			AccountID: accountsList[targetIdx].ID,
			Provider:  txAttempt.Provider,
			Amount:    txAttempt.Amount,
			Status:    "Paid",
			CreatedAt: time.Now(),
		}
		billsList = append(billsList, newBillRecord)

		updatedBillBytes, _ := json.MarshalIndent(billsList, "", "  ")
		_ = os.WriteFile("data/bills.json", updatedBillBytes, 0644)

		createNotification(accountsList[targetIdx].ID, fmt.Sprintf("Utility bill payment of $%d to %s was successfully processed.", txAttempt.Amount, txAttempt.Provider))
	}

	if txAttempt.Category == "bank_tx" {
		var txList []Transactions
		txBytes, err := os.ReadFile("data/transactions.json")
		if err == nil && len(txBytes) > 0 {
			json.Unmarshal(txBytes, &txList)
		}

		nextTxID := 1
		if len(txList) > 0 {
			nextTxID = txList[len(txList)-1].ID + 1
		}

		rSource := rand.NewSource(time.Now().UnixNano())
		rGen := rand.New(rSource)
		refString := fmt.Sprintf("TXN%08d", rGen.Intn(100000000))

		newTxRecord := Transactions{
			ID:              nextTxID,
			AccountID:       accountsList[targetIdx].ID,
			TransactionType: txAttempt.Type,
			Amount:          txAttempt.Amount,
			Sender:          txAttempt.AccountNumber,
			Recipient:       txAttempt.Recipient,
			Status:          "Completed",
			Reference:       refString,
			CreatedAt:       time.Now(),
		}
		txList = append(txList, newTxRecord)

		updatedTxBytes, _ := json.MarshalIndent(txList, "", "  ")
		_ = os.WriteFile("data/transactions.json", updatedTxBytes, 0644)

		/* ===== UPDATED NOTIFICATION LOGIC ====== */
		if txAttempt.Type == "transfer" {
			createNotification(accountsList[targetIdx].ID, fmt.Sprintf("Sent a transfer of $%d to account %s.", txAttempt.Amount, txAttempt.Recipient))
			if recipientIdx != -1 {
				// Notifies recipient exactly who sent it and how much
				createNotification(accountsList[recipientIdx].ID, fmt.Sprintf("Received an inbound transfer of $%d from account %s.", txAttempt.Amount, txAttempt.AccountNumber))
			}
		} else {
			createNotification(accountsList[targetIdx].ID, fmt.Sprintf("Successfully processed a %s of $%d.", txAttempt.Type, txAttempt.Amount))
		}
	}

	message := fmt.Sprintf("Transaction processed! Approved action: %s", txAttempt.Type)
	if txAttempt.Category == "bill_pay" {
		message = fmt.Sprintf("Bill payment of $%d to %s successful!", txAttempt.Amount, txAttempt.Provider)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     message,
		"new_balance": accountsList[targetIdx].Balance,
	})
}

// Combines, cleans, and dates ledger summaries
func handleTransactionLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	accountNumber := r.URL.Query().Get("account_number")
	if accountNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing account_number parameter"})
		return
	}

	accBytes, err := os.ReadFile("data/accounts.json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Database error accessing ledger."})
		return
	}

	var accountsList []AccountsData
	json.Unmarshal(accBytes, &accountsList)

	targetAccountID := -1
	for _, acc := range accountsList {
		if acc.AccountNumber == accountNumber {
			targetAccountID = acc.ID
			break
		}
	}

	masterHistory := []UnifiedLog{}

	var allTransactions []Transactions
	txBytes, err := os.ReadFile("data/transactions.json")
	if err == nil && len(txBytes) > 0 {
		json.Unmarshal(txBytes, &allTransactions)
	}

	for _, tx := range allTransactions {
		if tx.Sender == accountNumber || tx.Recipient == accountNumber {
			txType := tx.TransactionType
			if tx.TransactionType == "transfer" && tx.Recipient == accountNumber {
				txType = "transfer (Inbound)"
			}

			masterHistory = append(masterHistory, UnifiedLog{
				CreatedTime: tx.CreatedAt,
				Reference:   tx.Reference,
				Type:        txType,
				Amount:      tx.Amount,
				Status:      tx.Status,
			})
		}
	}

	if targetAccountID != -1 {
		var allBills []Bills
		billBytes, err := os.ReadFile("data/bills.json")
		if err == nil && len(billBytes) > 0 {
			json.Unmarshal(billBytes, &allBills)
		}

		for _, b := range allBills {
			if b.AccountID == targetAccountID {
				masterHistory = append(masterHistory, UnifiedLog{
					CreatedTime: b.CreatedAt,
					Reference:   fmt.Sprintf("BILL%05d", b.ID),
					Type:        fmt.Sprintf("Bill (%s)", b.Provider),
					Amount:      b.Amount,
					Status:      b.Status,
				})
			}
		}
	}

	for i := 0; i < len(masterHistory)-1; i++ {
		for j := 0; j < len(masterHistory)-i-1; j++ {
			if masterHistory[j].CreatedTime.Before(masterHistory[j+1].CreatedTime) {
				masterHistory[j], masterHistory[j+1] = masterHistory[j+1], masterHistory[j]
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(masterHistory)
}

// Drops backward-sorted list arrays of user notifications
func handleNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	accountNumber := r.URL.Query().Get("account_number")
	if accountNumber == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing account_number parameter"})
		return
	}

	accBytes, _ := os.ReadFile("data/accounts.json")
	var accountsList []AccountsData
	json.Unmarshal(accBytes, &accountsList)

	userID := -1
	for _, acc := range accountsList {
		if acc.AccountNumber == accountNumber {
			userID = acc.UserID
			break
		}
	}

	var allNotifications []Notifications
	notifBytes, err := os.ReadFile("data/notifications.json")
	if err == nil && len(notifBytes) > 0 {
		json.Unmarshal(notifBytes, &allNotifications)
	}

	userNotifications := []Notifications{}
	for i := len(allNotifications) - 1; i >= 0; i-- {
		if allNotifications[i].UserID == userID {
			userNotifications = append(userNotifications, allNotifications[i])
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userNotifications)
}

/* ======== HELPER FUNCTIONS ======= */

// Creates database line entries inside notifications.json
func createNotification(userID int, message string) {
	var notificationsList []Notifications
	fileBytes, err := os.ReadFile("data/notifications.json")
	if err == nil && len(fileBytes) > 0 {
		json.Unmarshal(fileBytes, &notificationsList)
	}

	nextID := 1
	if len(notificationsList) > 0 {
		nextID = notificationsList[len(notificationsList)-1].ID + 1
	}

	newNotification := Notifications{
		ID:        nextID,
		UserID:    userID,
		Message:   message,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
	notificationsList = append(notificationsList, newNotification)

	fileData, _ := json.MarshalIndent(notificationsList, "", "  ")
	_ = os.WriteFile("data/notifications.json", fileData, 0644)
}
