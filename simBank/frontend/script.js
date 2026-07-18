'use strict';

const API_BASE_URL = 'http://localhost:8080/api'; 

const authContainer = document.getElementById('auth-container');
const appContainer = document.getElementById('app-container');
const registerFields = document.getElementById('register-fields');
const authTitle = document.getElementById('auth-title');
const authSubtitle = document.getElementById('auth-subtitle');
const authSubmitBtn = document.getElementById('auth-submit-btn');
const toggleText = document.getElementById('auth-toggle-text');

const firstNameInput = document.getElementById('reg-firstname');
const lastNameInput = document.getElementById('reg-lastname');

let isLoginMode = true;

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
            alert(`Response: ${data.message}`);
            
            document.getElementById('user-display-name').innerText = data.name || "User";
            
            if (data.account_number) {
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
            alert(`Go Server Error: ${data.error}`);
        }
    } catch (error) {
        console.error("Connection failed:", error);
        alert("Cannot connect to the Go backend. Is your main.go running?");
    }
}

function showSection(sectionId) {
    const sections = document.querySelectorAll('.system-section');
    sections.forEach(section => section.classList.remove("active"));
    
    const target = document.getElementById(sectionId);
    if (target) target.classList.add("active");
}

function openPinModal() {
    document.querySelector('.pin-modal').style.display = 'flex';
}

function closePinModal() {
    const pinInput = document.querySelector('.pin-modal input');
    if (pinInput) pinInput.value = '';
    document.querySelector('.pin-modal').style.display = 'none';
}

function confirmTransaction() {
    alert("Transaction Processed Successfully!");
    closePinModal();
    showSection("dashboard-section");
}

function cancelTransaction() {
    closePinModal();
}

function handleLogout() {
    appContainer.classList.add('hidden');
    authContainer.classList.remove('hidden');
    document.getElementById('auth-form').reset();
}