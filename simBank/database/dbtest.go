package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func main() {
	host := "host_ip_address"
	port := 5432
	user := "user_name"
	password := "your_password"
	dbname := "database_name"

	connectionString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=require",
		user,
		password,
		host,
		port,
		dbname,
	)

	// Connect to PostgreSQL
	conn, err := pgx.Connect(context.Background(), connectionString)
	if err != nil {
		log.Fatal("Could not connect to database:", err)
	}

	defer conn.Close(context.Background())

	fmt.Println("Connected to PostgreSQL successfully!")

	// Simple test query
	var databaseName string

	err = conn.QueryRow(
		context.Background(),
		"SELECT current_database();",
	).Scan(&databaseName)

	if err != nil {
		log.Fatal("Query failed:", err)
	}

	fmt.Println("Connected database:", databaseName)
}
