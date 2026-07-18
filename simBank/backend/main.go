package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

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

func main() {
	os.MkdirAll("data", os.ModePerm)

	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)

	fmt.Println("Server starting on http://localhost:8080...")
	http.ListenAndServe(":8080", nil)
}

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
		w.Write([]byte(`{"error": "Registration rejected: All fields (First Name, Last Name, Email, Password) are strictly required."}`))
		return
	}

	var usersList []UserData
	fileBytes, err := os.ReadFile("data/users.json")
	if err == nil && len(fileBytes) > 0 {
		json.Unmarshal(fileBytes, &usersList)
	}

	nextUserID := 1
	if len(usersList) > 0 {
		nextUserID = usersList[len(usersList)-1].ID + 1
	}

	incomingData.ID = nextUserID
	incomingData.CreatedAt = time.Now()
	usersList = append(usersList, incomingData)

	fileData, _ := json.MarshalIndent(usersList, "", "  ")
	err = os.WriteFile("data/users.json", fileData, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to write user data file"}`))
		return
	}

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
		Balance:       1000,
		AccountType:   "Checking",
		Status:        "Active",
	}
	accountsList = append(accountsList, newAccount)

	accFileData, _ := json.MarshalIndent(accountsList, "", "  ")
	err = os.WriteFile("data/accounts.json", accFileData, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to create bank account linkage"}`))
		return
	}

	fmt.Printf("Registered User ID %d: %s %s. Created Account: %s\n",
		incomingData.ID, incomingData.FirstName, incomingData.LastName, generatedAccNum)

	fullName := incomingData.FirstName + " " + incomingData.LastName
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON := fmt.Sprintf(`{"message": "Registration saved successfully! Account %s opened.", "name": "%s"}`, generatedAccNum, fullName)
	w.Write([]byte(responseJSON))
}

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
		fmt.Printf("User %s (ID: %d) logged in successfully.\n", fullName, matchedUser.ID)
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
