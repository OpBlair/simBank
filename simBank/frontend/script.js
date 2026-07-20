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

/* ===== INITIALIZATION BLOCK ===== */
document.addEventListener('DOMContentLoaded', () => {
    // Auto-focus behavior for the segmented PIN setup boxes
    document.querySelectorAll('.pin-digits-row').forEach(row => {
        const inputs = row.querySelectorAll('.pin-box');
        
        inputs.forEach((input, index) => {
            // Automatically jump forward on typing a digit
            input.addEventListener('input', (e) => {
                if (e.target.value.length === 1 && index < inputs.length - 1) {
                    inputs[index + 1].focus();
                }
            });

            // Automatically jump back on pressing Backspace
            input.addEventListener('keydown', (e) => {
                if (e.key === 'Backspace' && e.target.value.length === 0 && index > 0) {
                    inputs[index - 1].focus();
                }
            });
        });
    });
});

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
            
            // Force a user to set up pin after registration.
            if (!isLoginMode) {
                authContainer.classList.add('hidden');
                appContainer.classList.remove('hidden');
                openSetPinModal();
            } else {
                // Regular login goes straight to dashboard
                authContainer.classList.add('hidden');
                appContainer.classList.remove('hidden');
                fetchAndDisplayNotifications();
                showSection("dashboard-section");
            }

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
        // First mark all notifications as read in database
        markNotificationsAsRead().then(() => {
            // Then fetch updated lists to clear active badge counts
            fetchAndDisplayNotifications();
        });
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
    loggedInAccount = "";
}

function openSetPinModal(){
    document.getElementById('setup-pin-modal').style.display = 'flex';
}

function closeSetPinModal(){
    document.getElementById('setup-pin-modal').style.display = 'none';
    document.querySelectorAll('#setup-pin-modal .pin-box').forEach(input => input.value = '');
}

/* ===== PIN SETUP MODULE ====== */

// Helper function to extract PIN values from the digit rows
function getPinFromRow(rowId) {
    let pin = "";
    document.querySelectorAll(`#${rowId} .pin-box`).forEach(input => {
        pin += input.value;
    });
    return pin;
}

async function saveAccountPin() {
    const pin = getPinFromRow('new-pin-row');
    const confirmPin = getPinFromRow('confirm-pin-row');

    // Validation
    if (pin.length !== 4 || confirmPin.length !== 4 || isNaN(pin) || isNaN(confirmPin)) {
        console.log("PIN must be exactly 4 digits.");
        return;
    }

    if (pin !== confirmPin) {
        console.log("PINs do not match. Please try again.");
        return;
    }

    const payload = {
        account_number: loggedInAccount,
        pin: pin
    };

    try {
        const response = await fetch(`${API_BASE_URL}/setup-pin`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });

        const data = await response.json();

        if (response.ok) {
            console.log("PIN successfully created!");
            closeSetPinModal();
            showSection("dashboard-section");
        } else {
            console.log(`Failed to save PIN: ${data.error}`);
        }
    } catch (error) {
        console.error("Setup PIN API failed:", error);
    }
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
        category: transactionType, // Directly maps to 'deposit', 'withdraw', or 'transfer'
        amount: amount,
        account_number: accountNumber,
        recipient: transactionType === 'transfer' ? recipient : ""
    };

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
            document.getElementById('user-display-balance').innerText = `$${Number(data.balance).toFixed(2)}`;
            
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
            fetchAndDisplayNotifications();
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
        category: 'pay_bill',
        amount: amount,
        account_number: accountNumber,
        provider: provider,
        recipient: ""
    };

    openPinModal(); 
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

// POST updates to target endpoint clearing is_read state items on active cluster
async function markNotificationsAsRead() {
    if (!loggedInAccount) return;

    try {
        await fetch(`${API_BASE_URL}/notifications/read`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ account_number: loggedInAccount })
        });
    } catch (error) {
        console.error("Failed to mark notifications as read:", error);
    }
}

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

            // Calculate only unread notifications from the total pool
            const unreadCount = data.filter(notif => !notif.is_read).length;

            const badge = document.getElementById('notif-badge');
            const activeSection = document.querySelector('.system-section.active');

            if (badge) {
                // If the user is currently viewing the notification panel, force the badge to hide/reset
                if (activeSection && activeSection.id === 'notifications-section') {
                    badge.innerText = '0';
                    badge.classList.add('hidden');
                } else if (unreadCount > 0) {
                    badge.innerText = unreadCount;
                    badge.classList.remove('hidden');
                } else {
                    badge.classList.add('hidden');
                }
            }

            if (data.length === 0) {
                listContainer.innerHTML = '<div class="no-notifications" style="padding: 10px; color: gray;">No notifications available.</div>';
                return;
            }

            data.forEach(notif => {
                const item = document.createElement('div');
                item.className = 'notification-item';
                item.style.borderBottom = '1px solid #eee';
                item.style.padding = '12px 6px';
                
                // Soft style distinction for unread items vs read items
                if (!notif.is_read) {
                    item.style.backgroundColor = '#f8fafc';
                } else {
                    item.style.backgroundColor = 'transparent';
                }
                
                const timeStr = new Date(notif.created_at).toLocaleString();

                item.innerHTML = `
                    <p style="margin: 0 0 4px 0; font-weight: ${notif.is_read ? '500' : '700'};">
                        ${notif.message} ${!notif.is_read ? '<span style="color: #2563eb; font-size: 10px; margin-left: 5px;">●</span>' : ''}
                    </p>
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