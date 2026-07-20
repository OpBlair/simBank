CREATE DATABASE simbank;
CREATE TABLE users (
    id SERIAL PRIMARY KEY,                    -- Go: UserData.ID
    first_name VARCHAR(255) NOT NULL,          -- Go: UserData.FirstName
    last_name VARCHAR(255) NOT NULL,           -- Go: UserData.LastName
    email VARCHAR(255) NOT NULL UNIQUE,        -- Go: UserData.Email
    password VARCHAR(255) NOT NULL,            -- Go: UserData.Password
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP            -- Go: UserData.CreatedAt
);

CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,                    -- Go: AccountsData.ID
    user_id INT NOT NULL REFERENCES users(id), -- Go: AccountsData.UserID
    acc_number VARCHAR(20) UNIQUE NOT NULL,    -- Go: AccountsData.AccountNumber
    balance BIGINT NOT NULL DEFAULT 0,            -- Go: AccountsData.Balance
    acc_type VARCHAR(50) NOT NULL DEFAULT 'Checking',             -- Go: AccountsData.AccountType
    status VARCHAR(50) NOT NULL DEFAULT 'Active',               -- Go: AccountsData.Status
    pin VARCHAR(255) NOT NULL,                      -- Go: AccountsData.Pin
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,                    -- Go: Transactions.ID
    account_id INT NOT NULL REFERENCES accounts(id), -- Go: Transactions.AccountID
    transaction_type VARCHAR(50) NOT NULL,     -- Go: Transactions.TransactionType
    amount BIGINT NOT NULL,                       -- Go: Transactions.Amount
    sender VARCHAR(50) NULL,                   -- Go: Transactions.Sender
    recipient VARCHAR(50) NULL,                -- Go: Transactions.Recipient
    status VARCHAR(50) NOT NULL,               -- Go: Transactions.Status
    reference VARCHAR(100) UNIQUE NOT NULL,    -- Go: Transactions.Reference
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP              -- Go: Transactions.CreatedAt
);

CREATE TABLE bills (
    id SERIAL PRIMARY KEY,                    -- Go: Bills.ID
    account_id INT NOT NULL REFERENCES accounts(id), -- Go: Bills.AccountID
    provider VARCHAR(255) NOT NULL,            -- Go: Bills.Provider
    amount BIGINT NOT NULL,                       -- Go: Bills.Amount
    status VARCHAR(50) NOT NULL,               -- Go: Bills.Status
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP             -- Go: Bills.CreatedAt
);

CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,                    -- Go: Notifications.ID
    user_id INT NOT NULL REFERENCES users(id), -- Go: Notifications.UserID
    message TEXT NOT NULL,                     -- Go: Notifications.Message
    is_read BOOLEAN NOT NULL DEFAULT FALSE,    -- Go: Notifications.IsRead
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP              -- Go: Notifications.CreatedAt
);
