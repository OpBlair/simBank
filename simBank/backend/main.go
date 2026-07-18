package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// UserData handles the full profile matrix
type UserData struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

func main() {
	// Ensure the data directory exists right away
	os.MkdirAll("data", os.ModePerm)

	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)

	fmt.Println("Server starting on http://localhost:8080...")
	http.ListenAndServe(":8080", nil)
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	// Setup CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		return
	}

	var incomingData UserData
	json.NewDecoder(r.Body).Decode(&incomingData)

	// 1. Read existing users array from file
	var usersList []UserData
	fileBytes, err := os.ReadFile("data/users.json")

	// If the file exists, parse its current contents into our list array
	if err == nil && len(fileBytes) > 0 {
		json.Unmarshal(fileBytes, &usersList)
	}

	// 2. Append the new user to our array list
	usersList = append(usersList, incomingData)

	// 3. Convert the updated array list back to pretty JSON text
	fileData, _ := json.MarshalIndent(usersList, "", "  ")

	// 4. Save the updated list array back into the storage file path
	err = os.WriteFile("data/users.json", fileData, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to write user data file on host storage"}`))
		return
	}

	fmt.Printf("Registered User: %s %s Saved to list.\n", incomingData.FirstName, incomingData.LastName)

	fullName := incomingData.FirstName + " " + incomingData.LastName
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON := fmt.Sprintf(`{"message": "Registration saved successfully!", "name": "%s"}`, fullName)
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

	// 1. Read the saved array file from storage disk
	fileBytes, err := os.ReadFile("data/users.json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "No registered users found. Please register first."}`))
		return
	}

	// 2. Parse the file data into a structured slice array list
	var usersList []UserData
	json.Unmarshal(fileBytes, &usersList)

	// 3. Loop through the array to find a match
	var userFound bool
	var matchedUser UserData

	for _, user := range usersList {
		if user.Email == loginAttempt.Email && user.Password == loginAttempt.Password {
			userFound = true
			matchedUser = user
			break
		}
	}

	// 4. Send back the appropriate response validation status
	if userFound {
		fullName := matchedUser.FirstName + " " + matchedUser.LastName
		fmt.Printf("User %s logged in successfully.\n", fullName)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		responseJSON := fmt.Sprintf(`{"message": "Login verified!", "name": "%s"}`, fullName)
		w.Write([]byte(responseJSON))
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Invalid email or password credentials."}`))
	}
}
