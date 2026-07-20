CREATE DATABASE simbank;

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    acc_number VARCHAR(20) UNIQUE NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    acc_type VARCHAR(50) NOT NULL DEFAULT 'Checking',
    status VARCHAR(50) NOT NULL DEFAULT 'Active',
    pin VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE transactions (
    id SERIAL PRIMARY KEY,
    account_id INT NOT NULL REFERENCES accounts(id),
    transaction_type VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    sender VARCHAR(50) NULL,
    recipient VARCHAR(50) NULL,
    status VARCHAR(50) NOT NULL,
    reference VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE providers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    account_number VARCHAR(100) NOT NULL,
    account_holder VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default providers with legit 10-digit account numbers
INSERT INTO providers (name, account_number, account_holder, is_active) VALUES
('Electricity', '1000567890', 'National Power Company Ltd', TRUE),
('Water', '1001234567', 'Water Authority Services', TRUE),
('Internet', '1002789456', 'Broadband Communications Inc', TRUE);

CREATE TABLE bills (
    id SERIAL PRIMARY KEY,
    account_id INT NOT NULL REFERENCES accounts(id),
    provider_id INT NOT NULL REFERENCES providers(id),
    amount BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE notifications (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);