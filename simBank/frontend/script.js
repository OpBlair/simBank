'use strict';

/* ===== DEBUGGING BARS ====== */
/*
// UNCOMMENT THIS IF THE PAGE KEEPS REFRESHING AND DESTROYING YOUR STATE
window.onbeforeunload = function() {
    console.trace("The page is reloading right now! Trace back what called it:");
    debugger;
    return "Stop!"; 
};*/

/* ===== CONFIG & GLOBAL STATE ====== */
const API_BASE_URL = 'http://localhost:8080/api'; // Go backend port

/* ===== DOM ELEMENTS ====== */
const authContainer = document.getElementById('auth-container');
const appContainer = document.getElementById('app-container');
const registerFields = document.getElementById('register-fields');
const authTitle = document.getElementById('auth-title');
const authSubtitle = document.getElementById('auth-subtitle');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const toggleText = document.getElementById('auth-toggle-text');
const firstNameInput = document.getElementById('reg-firstname');
const lastNameInput = document.getElementById('reg-lastname');

// Session data resets on page refresh
let isLoginMode = true;
let loggedInAccount = "";
let pendingTransaction = null; // Staged data before PIN confirmation

/* ===== AUTHENTICATION MODULE ====== */

// Flips UI between login and registration forms
function toggleAuthMode(e) {
    e.preventDefault(); 
    isLoginMode = !isLoginMode;

    if (!isLoginMode) {
        authTitle.innerText = "Create an Account";
        authSubtitle.innerText = "Join SimBank HA Cluster environment";
        registerFields.classList.remove('hidden');
        authSubmitBtn.innerText = "Register";
        toggleText.innerHTML = `Already have an account? <a href="#" onclick="toggleAuthMode(event)">Login here</a>`;

        firstNameInput.required = true;
        lastNameInput.required = true;
    } else {
        authTitle.innerText = "Login to SimBank";
        authSubtitle.innerText = "Access your high-availability secure portal";
        registerFields.classList.add('hidden');
        authSubmitBtn.innerText = "Login";
        toggleText.innerHTML = `Don't have an account? <a href="#" onclick="toggleAuthMode(event)">Register here</a>`;
        
        firstNameInput.required = false;
        lastNameInput.required = false;
    }
}

// Sends login/register data to backend
async function handleAuth(e) {
    e.preventDefault(); 
    
    const email = document.getElementById('auth-email').value;
    const password = document.getElementById('auth-password').value;
    const endpoint = isLoginMode ? `${API_BASE_URL}/login` : `${API_BASE_URL}/register`;
    
    const payload = {
        email: email,
        password: password
    };
    
    if (!isLoginMode) {
        payload.first_name = firstNameInput.value;
        payload.last_name = lastNameInput.value;
    }

    try {
        const response = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        const data = await response.json();

        if (response.ok) {
            console.log(`Response: ${data.message}`);
            
            document.getElementById('user-display-name').innerText = data.name || "User";
            
            if (data.account_number) {
                loggedInAccount = data.account_number; // Save globally for other APIs
                document.getElementById('user-display-account').innerText = data.account_number;
                document.getElementById('user-display-balance').innerText = `$${Number(data.balance).toFixed(2)}`;
            } else {
                document.getElementById('user-display-account').innerText = "Please log in to view account";
                document.getElementById('user-display-balance').innerText = "$0.00";
            }
            
            authContainer.classList.add('hidden');
            appContainer.classList.remove('hidden');
            showSection("dashboard-section");
        } else {
            console.log(`Go Server Error: ${data.error}`);
        }
    } catch (error) {
        console.error("Connection failed:", error);
        console.log("Cannot connect to the Go backend. Is your main.go running?");
    }
}

/* ===== NAVIGATION & TABS ====== */

// Simple SPA tab switcher
function showSection(sectionId) {
    const sections = document.querySelectorAll('.system-section');
    sections.forEach(section => section.classList.remove("active"));
    
    const target = document.getElementById(sectionId);
    if (target) target.classList.add("active");

    // Fetch fresh data when user clicks the tab
    if (sectionId === 'transaction-logs-section') {
        fetchAndDisplayLogs();
    }

    if (sectionId === 'notifications-section') {
        fetchAndDisplayNotifications();
    }
}

function openPinModal() {
    document.querySelector('.pin-modal').style.display = 'flex';
}

function closePinModal() {
    const pinInput = document.querySelector('.pin-modal input');
    if (pinInput) pinInput.value = ''; // Reset input so pin isn't cached in UI
    document.querySelector('.pin-modal').style.display = 'none';
}

function cancelTransaction() {
    closePinModal();
}

function handleLogout() {
    appContainer.classList.add('hidden');
    authContainer.classList.remove('hidden');
    document.getElementById('auth-form').reset();
}

/* ===== TRANSACTION MODULE ====== */

// Shows/hides recipient field depending on transaction type
function handleTxTypeChange() {
    const txType = document.getElementById('tx-type').value;
    const recipientField = document.getElementById('recipient-field');
    
    if (txType === 'transfer') {
        recipientField.classList.remove('hidden');
    } else {
        recipientField.classList.add('hidden');
        document.getElementById('tx-recipient').value = '';
    }
}

// Stage 1: Validate input fields and trigger PIN modal
function prepareTransaction(){
    const transactionType = document.getElementById('tx-type').value;
    const amount = parseInt(document.getElementById('tx-amount').value, 10);
    const accountNumber = loggedInAccount;
    const recipient = document.getElementById('tx-recipient').value;

    if(isNaN(amount) || amount <= 0){
        console.log("Please enter a valid amount greater than 0");
        return;
    }

    if(transactionType === 'transfer' && !recipient){
        console.log("please enter a recipient account number");
        return;
    }

    pendingTransaction = {
        category: 'bank_tx',
        type: transactionType,
        amount: amount,
        account_number: accountNumber,
        recipient: transactionType === 'transfer' ? recipient : ""
    }

    openPinModal();
}

// Stage 2: Combine staged object with PIN and POST to database
async function confirmTransaction(e) {
    if (e) {
        e.preventDefault();
        e.stopPropagation();
    }

    const pinInput = document.querySelector('.pin-modal input').value;

    if(pinInput === '' || pinInput.length != 4){
        console.log("Please enter a valid digit pin");
        return;
    }

    const payload = {
        ...pendingTransaction,
        pin: pinInput
    }

    try{
        const response = await fetch(`${API_BASE_URL}/transaction`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json'},
            body: JSON.stringify(payload) 
        });

        const data = await response.json();

        if(response.ok){
            console.log(`Success ${data.message}`);
            document.getElementById('user-display-balance').innerText = `$${Number(data.new_balance).toFixed(2)}`;
            
            // Clean UI input elements explicitly 
            const txAmountInput = document.getElementById('tx-amount');
            if (txAmountInput) txAmountInput.value = '';

            const txRecipientInput = document.getElementById('tx-recipient');
            if (txRecipientInput) txRecipientInput.value = '';

            const billAmountInput = document.getElementById('bill-amount');
            if (billAmountInput) billAmountInput.value = '';

            const recipientField = document.getElementById('recipient-field');
            if (recipientField) recipientField.classList.add('hidden');

            const txTypeSelect = document.getElementById('tx-type');
            if (txTypeSelect) txTypeSelect.value = 'deposit'; 

            pendingTransaction = null; 
            
            closePinModal();
            fetchAndDisplayLogs();
            showSection("dashboard-section");
        } else {
            console.log(`Transaction Denied: ${data.error}`);
        }
    } catch (error) {
        console.error("Transaction API failed:", error);
    }
}

// Stages utility bill processing as a deduction
function prepareBillPayment() {
    const provider = document.getElementById('bill-type').value;
    const amount = parseInt(document.getElementById('bill-amount').value, 10);
    const accountNumber = loggedInAccount;

    if (isNaN(amount) || amount <= 0) {
        console.log("Please enter a valid bill amount greater than 0");
        return;
    }

    pendingTransaction = {
        category: 'bill_pay',
        type: 'withdraw', 
        amount: amount,
        account_number: accountNumber,
        provider: provider,
        recipient: ""
    };

    openPinModal(); // Reuses same PIN workflow
}

/* ===== TRANSACTION LOGS MODULE ====== */

function formatTimestamp(isoString) {
    if (!isoString) return 'N/A';
    const date = new Date(isoString);
    return date.toLocaleString(); 
}

// Populates transaction log tables dynamically
async function fetchAndDisplayLogs() {
    const tableBody = document.getElementById('logs-table-body');
    if (!tableBody || !loggedInAccount) return;

    try {
        const response = await fetch(`${API_BASE_URL}/transactionLogs?account_number=${loggedInAccount}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });

        const data = await response.json();

        if (response.ok && Array.isArray(data)) {
            tableBody.innerHTML = ''; 

            if (data.length === 0) {
                tableBody.innerHTML = `<tr><td colspan="5" style="text-align: center;">No transactions found.</td></tr>`;
                return;
            }

            data.forEach(log => {
                const row = document.createElement('tr');
                const statusClass = log.status ? log.status.toLowerCase() : 'unknown';

                row.innerHTML = `
                    <td>${formatTimestamp(log.timestamp || log.time)}</td>
                    <td><code>${log.reference || log.id || 'N/A'}</code></td>
                    <td><span class="tx-type-badge">${log.type ? log.type.toUpperCase() : 'UNKNOWN'}</span></td>
                    <td>$${Number(log.amount).toFixed(2)}</td>
                    <td><span class="status-${statusClass}">${log.status || 'COMPLETED'}</span></td>
                `;
                
                tableBody.appendChild(row);
            });
        } else {
            console.log(`Failed to fetch logs: ${data.error || 'Unknown error'}`);
            tableBody.innerHTML = `<tr><td colspan="5" style="text-align: center; color: red;">Error loading logs.</td></tr>`;
        }
    } catch (error) {
        console.error("Logs API connection failed:", error);
    }
}

/* ===== NOTIFICATIONS MODULE ====== */

// Populates notifications element feed dynamically
async function fetchAndDisplayNotifications() {
    const listContainer = document.getElementById('notifications-list');
    if (!listContainer || !loggedInAccount) return;

    try {
        const response = await fetch(`${API_BASE_URL}/notifications?account_number=${loggedInAccount}`, {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        });

        const data = await response.json();

        if (response.ok && Array.isArray(data)) {
            listContainer.innerHTML = ''; 

            if (data.length === 0) {
                listContainer.innerHTML = '<div class="no-notifications" style="padding: 10px; color: gray;">No notifications available.</div>';
                return;
            }

            data.forEach(notif => {
                const item = document.createElement('div');
                item.className = 'notification-item';
                item.style.borderBottom = '1px solid #eee';
                item.style.padding = '12px 6px';
                
                const timeStr = new Date(notif.created_at).toLocaleString();

                item.innerHTML = `
                    <p style="margin: 0 0 4px 0; font-weight: 500;">${notif.message}</p>
                    <small style="color: gray;">${timeStr}</small>
                `;
                listContainer.appendChild(item);
            });
        } else {
            console.log("Failed to fetch notifications:", data.error);
        }
    } catch (error) {
        console.error("Notifications API failed:", error);
    }
}