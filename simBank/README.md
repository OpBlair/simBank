# SimBank - High Availability Banking Simulator

SimBank is a full-stack banking simulator built to explore backend development, distributed systems concepts, and high availability. The project combines a vanilla HTML, CSS, and JavaScript frontend with a Go backend and evolves from a simple JSON-based storage layer to PostgreSQL with replication.

The goal of the project is not only to simulate common banking operations but also to understand how modern backend systems communicate with clients, manage data, and maintain availability.

## Development Philosophy

This project is being built incrementally to better understand the technologies involved. Features are implemented step by step, beginning with a JSON-based persistence layer before progressing to PostgreSQL, replication, and high-availability concepts.

## Tech Stack

Frontend: Clean Semantic HTML5, Vanilla JavaScript (use strict runtime protection), and modern CSS3 (Flexbox/Grid elements).

Backend: Go (Golang) REST API engine managing native HTTP routing and manual CORS control policies.

Storage Layer: Local JSON array persistence layer (data/users.json).

## File Structure
```
plaintext
├── data/
│   └── users.json     # Local database array storing registered profiles
├── index.html         # Single Page Application core frame structure
├── style.css          # Visual layout definitions & register page theme overrides
├── script.js          # Client-side API payload handler & state controllers
├── main.go            # Go server router, CORS controller, and verification loops
└── README.md          # Technical system documentation file
```

## How to Initialize and Run the Project

Follow these steps to get your environment initialized and up and running from scratch:
1. Initialize the Go Module

Open your terminal inside your main project directory and run the following command to initialize your Go environment:
```
Bash

go mod init simbank
```
2. Set Up the Data Directory

Ensure Git ignores your runtime local user data so it isn't pushed to GitHub. Create a .gitignore file in your root folder and add:

```
plaintext
data/
```
(The Go server will automatically generate the data/ folder and users.json file if they do not exist when a user registers).
3. Launch the Backend Server

Start your Go API server by running:
Bash

go run main.go

You should see a message stating: Server starting on http://localhost:8080...
4. Open the Frontend

Simply open your index.html file in any modern modern web browser, or use a code editor extension like Live Server to launch it.

## Project Roadmap

[x] Modern frontend layout prototype

[x] Multi-user registration array framework (Local JSON persistence)

[x] Secure multi-account login authentication & input validation

[ ] Wallet functionalities (Deposit & Withdrawal mechanisms)

[ ] Peer-to-peer money transfers

[ ] Dynamic transaction history logs binding

[ ] Utility bill payment module

[ ] PostgreSQL relational database integration

[ ] High Availability (HA) node cluster simulation