# SimBank

SimBank is a simple banking application built with Go, HTML, CSS and JavaScript. The project started as a way to learn how a frontend communicates with a backend before moving on to databases and high availability concepts.

At the moment the application stores data in JSON files, but the next step is replacing them with PostgreSQL.

## Features

- User registration
- User login
- Deposit money
- Withdraw money
- Transfer money between accounts
- Pay utility bills
- Transaction history
- Notifications

## Technologies Used

### Frontend
- HTML
- CSS
- Vanilla JavaScript

### Backend
- Go (Golang)
- Go net/http package

### Storage
- JSON files

## Project Structure

```text
.
├── backend/
│   ├── main.go
│   └── data/
│       ├── accounts.json
│       ├── bills.json
│       ├── notifications.json
│       ├── transactions.json
│       └── users.json
├── frontend/
│   ├── index.html
│   ├── style.css
│   └── script.js
├── go.mod
└── README.md
```

## Running the Project

Clone the repository.

Go into the backend folder.

```bash
cd backend
```

Run the Go server.

```bash
go run main.go
```

The backend will start on:

```
http://localhost:8080
```

Open the frontend by opening `frontend/index.html` in your browser, or use the Live Server extension in VS Code.

## API Endpoints

| Method | Endpoint | Description |
|---------|----------|-------------|
| POST | /api/register | Register a new user |
| POST | /api/login | Login |
| POST | /api/transaction | Deposit, withdraw, transfer and bill payment |
| GET | /api/transactionLogs | View transaction history |
| GET | /api/notifications | View notifications |

## Current Data Storage

The application currently stores information in JSON files located in:

```
backend/data/
```

These files act as a simple database while learning backend development.

- users.json
- accounts.json
- transactions.json
- bills.json
- notifications.json

## Future Improvements

- [x] Registration
- [x] Login
- [x] Deposit
- [x] Withdrawal
- [x] Money transfer
- [x] Bill payment
- [x] Transaction history
- [x] Notifications
- [ ] PostgreSQL integration
- [ ] Database replication
- [ ] High availability setup
- [ ] Docker support

## Author

Built as a learning project to understand Go backend development, REST APIs, and banking system concepts.
