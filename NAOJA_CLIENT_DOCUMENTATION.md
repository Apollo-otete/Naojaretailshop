# Naoja Ventures — Retail Shop Platform
## Complete System Documentation & Go-Live Guide

> **Version:** 1.0.0 | **Prepared for:** Naoja Ventures, Lurambi, Kakamega, Kenya  
> **Platform:** Full-stack E-Commerce (React + Node.js + PostgreSQL + MongoDB)  
> **Payment Gateway:** Safaricom Lipa Na M-Pesa (Till No. **4149288**)

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Technology Stack](#2-technology-stack)
3. [Project Directory Structure](#3-project-directory-structure)
4. [Prerequisites & Installation](#4-prerequisites--installation)
   - [Step 1 — Install Required Software](#step-1--install-required-software)
   - [Step 2 — Clone & Setup Databases](#step-2--clone--setup-databases)
   - [Step 3 — Configure Environment Variables](#step-3--configure-environment-variables)
   - [Step 4 — Start the Application](#step-4--start-the-application)
5. [Safaricom Daraja M-Pesa Integration Guide](#5-safaricom-daraja-m-pesa-integration-guide)
   - [What is Daraja API?](#what-is-daraja-api)
   - [Step A — Register on Safaricom Developer Portal](#step-a--register-on-safaricom-developer-portal)
   - [Step B — Create a Daraja Application](#step-b--create-a-daraja-application)
   - [Step C — Get Your API Credentials (Sandbox)](#step-c--get-your-api-credentials-sandbox)
   - [Step D — Testing with Sandbox (Development)](#step-d--testing-with-sandbox-development)
   - [Step E — Go Live (Production)](#step-e--go-live-production)
   - [How STK Push Works End-to-End](#how-stk-push-works-end-to-end)
   - [M-Pesa Callback Handling Explained](#m-pesa-callback-handling-explained)
   - [Common M-Pesa Error Codes](#common-m-pesa-error-codes)
6. [Admin Portal Guide](#6-admin-portal-guide)
7. [Database Schema Reference](#7-database-schema-reference)
8. [API Endpoint Reference](#8-api-endpoint-reference)
9. [Production Deployment (VPS/Cloud Server)](#9-production-deployment-vpscloud-server)
   - [Server Preparation](#server-preparation)
   - [Deploy Backend](#deploy-backend)
   - [Deploy Frontend](#deploy-frontend)
   - [Nginx + HTTPS Configuration](#nginx--https-configuration)
10. [Security Measures](#10-security-measures)
11. [Troubleshooting Guide](#11-troubleshooting-guide)
12. [Quick Reference Cheatsheet](#12-quick-reference-cheatsheet)

---

## 1. System Overview

**Naoja Retail Shop** is a production-grade e-commerce platform built for **Naoja Ventures**, an electrical and electronics retailer based in Lurambi, Kakamega, Kenya.

### What the System Does

| Feature | Description |
|---|---|
| 🛍️ Product Catalog | 14 specialized product categories (Mobile Phones, Audio, TVs, etc.) |
| 🔍 Search & Browse | Real-time product search and category filtering |
| 🛒 Shopping Cart | Persistent cart with quantity management |
| 💳 M-Pesa Checkout | Automated Lipa Na M-Pesa STK Push payments |
| 📦 Order Tracking | Customer order status polling every 3 seconds |
| 🏪 Pickup / Delivery | Store pickup or local delivery fulfillment options |
| 👤 Admin Dashboard | Secure admin portal for orders, products, and analytics |
| ⭐ Product Reviews | Customer reviews with admin approval workflow |
| 📧 Newsletter | Email subscriber management |
| 📩 Contact Forms | Customer message inbox in admin panel |

### Business Details Embedded in System

| Item | Value |
|---|---|
| Business Name | Naoja Ventures |
| Location | Lurambi, Kakamega, Kenya |
| M-Pesa Till Number | **4149288** |
| Default Admin Email | admin@naojaventures.com |
| Default Admin Password | **admin123** *(change immediately on first login)* |

---

## 2. Technology Stack

```
┌─────────────────────────────────────────────────────────────┐
│                     CLIENT BROWSER                          │
│              React 18 + Vite + Tailwind CSS                 │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTPS (via Nginx)
┌─────────────────────────▼───────────────────────────────────┐
│                   NGINX REVERSE PROXY                        │
│         SSL Termination + Static File Serving               │
└──────────┬──────────────────────────────────────────────────┘
           │                             │
    /api/* → Backend            /* → Frontend dist/
           │
┌──────────▼──────────────────────────────────────────────────┐
│              NODE.JS + EXPRESS API (Port 5000)               │
│    Auth · Products · Orders · M-Pesa · Analytics            │
└──────┬───────────────────────────┬──────────────────────────┘
       │                           │
┌──────▼──────┐           ┌────────▼────────┐
│ PostgreSQL  │           │    MongoDB       │
│  Port 5432  │           │   Port 27017     │
│─────────────│           │─────────────────│
│ orders      │           │ products         │
│ subscribers │           │ categories       │
│ messages    │           │ reviews          │
│ admins      │           │                  │
└─────────────┘           └─────────────────┘
                                  ↕
┌─────────────────────────────────────────────────────────────┐
│            SAFARICOM DARAJA API (External)                   │
│   OAuth 2.0 Token · STK Push · C2B Callback Webhook        │
└─────────────────────────────────────────────────────────────┘
```

| Layer | Technology | Version |
|---|---|---|
| Frontend Framework | React | 18 |
| Build Tool | Vite | 7 |
| CSS Framework | Tailwind CSS | 3.4 |
| Backend Runtime | Node.js | 18+ |
| Backend Framework | Express.js | 5.x |
| Relational Database | PostgreSQL | 15 |
| Document Database | MongoDB | 6 |
| Authentication | JSON Web Tokens (JWT) | — |
| Payment Gateway | Safaricom Daraja API | v1 |
| Process Manager | PM2 | 5.x |
| Reverse Proxy | Nginx | 1.24+ |
| SSL Certificates | Let's Encrypt (Certbot) | — |

---

## 3. Project Directory Structure

```
Naojaretailshop/
├── backend/
│   ├── src/
│   │   ├── config/
│   │   │   ├── db.js              # Connects to PostgreSQL & MongoDB
│   │   │   └── initPg.js          # Auto-creates PostgreSQL tables on startup
│   │   ├── middleware/
│   │   │   └── auth.js            # JWT token verification middleware
│   │   ├── migrations/
│   │   │   └── naoja_schema.sql   # Full PostgreSQL table definitions
│   │   ├── models/
│   │   │   ├── Product.js         # MongoDB Product schema (Mongoose)
│   │   │   ├── Category.js        # MongoDB Category schema
│   │   │   └── Review.js          # MongoDB Review schema
│   │   ├── routes/
│   │   │   ├── auth.js            # POST /api/auth/login, /api/auth/verify
│   │   │   ├── mpesa.js           # M-Pesa STK Push, Callback, Query
│   │   │   └── index.js           # Products, Orders, Analytics, etc.
│   │   └── scripts/
│   │       ├── seed.js            # Seed admin user + sample products
│   │       └── full-sync.js       # Sync utility script
│   ├── uploads/                   # Product image storage directory
│   ├── docker-compose.yml         # Local DB services (PostgreSQL + MongoDB)
│   ├── server.js                  # Application entry point
│   ├── package.json
│   └── .env.example               # Template for environment variables
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Header.jsx         # Navigation bar + cart icon + search
│   │   │   ├── Footer.jsx         # Site footer with business info
│   │   │   └── ProductCard.jsx    # Product listing card component
│   │   ├── lib/
│   │   │   ├── api.jsx            # All API call functions
│   │   │   ├── cart.jsx           # Cart Context (React Context API)
│   │   │   └── fulfillment.jsx    # Delivery/pickup fulfillment logic
│   │   ├── pages/
│   │   │   ├── HomePage.jsx       # Main storefront landing page
│   │   │   ├── CategoryPage.jsx   # Category product listing
│   │   │   ├── ProductPage.jsx    # Single product detail view
│   │   │   ├── CartPage.jsx       # Shopping cart review
│   │   │   ├── CheckoutPage.jsx   # Checkout + M-Pesa STK Push flow
│   │   │   ├── AccountPage.jsx    # Customer account / order history
│   │   │   ├── AdminDashboard.jsx # Admin management panel
│   │   │   └── AdminLogin.jsx     # Admin login page
│   │   ├── App.jsx                # Route definitions + auth guards
│   │   └── main.jsx               # React entry point
│   ├── index.html
│   ├── vite.config.js             # Vite build config + dev proxy
│   ├── tailwind.config.js
│   └── package.json
├── README.md
├── NAOJA_CLIENT_DOCUMENTATION.md  # This file
└── package.json
```

---

## 4. Prerequisites & Installation

### Step 1 — Install Required Software

#### Node.js (v18 or higher)
```bash
# Ubuntu/Debian
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# Verify
node --version   # v18.x.x or higher
npm --version    # v9.x.x or higher
```

#### Docker & Docker Compose (for local databases)
```bash
sudo apt-get update
sudo apt-get install -y docker.io docker-compose
sudo systemctl start docker && sudo systemctl enable docker
sudo usermod -aG docker $USER
```

---

### Step 2 — Clone & Setup Databases

```bash
# Clone the project
git clone https://github.com/Apollo-otete/Naojaretailshop.git
cd Naojaretailshop

# Start PostgreSQL + MongoDB via Docker
cd backend
docker-compose up -d

# Verify containers are running
docker ps
```

#### Install Dependencies
```bash
# Backend
cd backend && npm install

# Frontend
cd ../frontend && npm install
```

---

### Step 3 — Configure Environment Variables

#### Backend (`backend/.env`)
```bash
cd backend
cp .env.example .env
nano .env
```

```dotenv
# ─── Server ───────────────────────────────────
PORT=5000
NODE_ENV=development
FRONTEND_URL=http://localhost:5173

# ─── PostgreSQL ───────────────────────────────
PG_HOST=localhost
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=postgres
PG_DATABASE=naoja_shop

# ─── MongoDB ──────────────────────────────────
MONGODB_URI=mongodb://localhost:27017/naoja_shop

# ─── JWT Secret ───────────────────────────────
# Generate with: node -e "console.log(require('crypto').randomBytes(64).toString('hex'))"
JWT_SECRET=REPLACE_WITH_YOUR_64_CHAR_RANDOM_SECRET

# ─── Safaricom Daraja M-Pesa ──────────────────
# For testing (sandbox):
MPESA_BASE_URL=https://sandbox.safaricom.co.ke
# For production (go-live):
# MPESA_BASE_URL=https://api.safaricom.co.ke

MPESA_CONSUMER_KEY=YOUR_CONSUMER_KEY_FROM_DARAJA_PORTAL
MPESA_CONSUMER_SECRET=YOUR_CONSUMER_SECRET_FROM_DARAJA_PORTAL
MPESA_SHORTCODE=4149288
MPESA_PASSKEY=YOUR_LIPA_NA_MPESA_PASSKEY_FROM_DARAJA_PORTAL

# Must be a PUBLIC HTTPS URL (use ngrok for local testing)
MPESA_CALLBACK_URL=https://your-domain.com/api/mpesa/callback
```

#### Frontend (`frontend/.env`)
```bash
cd ../frontend
cp .env.example .env
```
```dotenv
# Leave blank when using Vite dev proxy
VITE_API_URL=
```

---

### Step 4 — Start the Application

```bash
# Terminal 1 — Backend
cd backend && npm run dev
# ✅ Server running on http://localhost:5000

# Terminal 2 — Frontend
cd frontend && npm run dev
# Local: http://localhost:5173

# Seed database (first time only)
cd backend && node src/scripts/seed.js
```

**Admin Login:**
- URL: `http://localhost:5173/admin`
- Email: `admin@naojaventures.com`
- Password: `admin123` ← **Change this immediately**

---

## 5. Safaricom Daraja M-Pesa Integration Guide

### What is Daraja API?

**Daraja** is Safaricom's official developer API platform. The Naoja system uses **Lipa Na M-Pesa (STK Push)** which:
1. Sends an automatic payment prompt to the customer's phone
2. Customer enters their M-Pesa PIN
3. Safaricom sends payment confirmation to the system automatically

---

### Step A — Register on Safaricom Developer Portal

1. Go to: **https://developer.safaricom.co.ke**
2. Click **"Sign Up"** → fill in your details
3. Verify your email address

---

### Step B — Create a Daraja Application

1. Log in → click **"My Apps"** → **"+ Add a New App"**
2. Fill in:
   - **App Name:** `Naoja Retail Shop`
   - **App Description:** `E-commerce payment integration for Naoja Ventures`
3. Check these APIs:
   - ✅ **Lipa Na M-Pesa Sandbox** (testing)
   - ✅ **Lipa Na M-Pesa Online** (production)
4. Click **"Create App"**

---

### Step C — Get Your API Credentials (Sandbox)

From your app page:

| Credential | Where to Find | Where to Put It |
|---|---|---|
| **Consumer Key** | App detail page | `MPESA_CONSUMER_KEY` in `.env` |
| **Consumer Secret** | App detail page | `MPESA_CONSUMER_SECRET` in `.env` |
| **Passkey** | APIs → Lipa Na M-Pesa Sandbox | `MPESA_PASSKEY` in `.env` |

> **Sandbox shortcode is:** `174379` (NOT your real till number during testing)

---

### Step D — Testing with Sandbox (Development)

#### Setup ngrok (to receive callbacks locally)
```bash
# Download and install ngrok
wget https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-amd64.tgz
tar -xvzf ngrok-v3-stable-linux-amd64.tgz && sudo mv ngrok /usr/local/bin/

# Add your ngrok auth token (sign up at ngrok.com)
ngrok config add-authtoken YOUR_NGROK_AUTHTOKEN

# Start tunnel (keep this running while testing)
ngrok http 5000
# Output: https://abc123.ngrok-free.app -> http://localhost:5000
```

Update `backend/.env` with the ngrok URL:
```dotenv
MPESA_BASE_URL=https://sandbox.safaricom.co.ke
MPESA_SHORTCODE=174379
MPESA_CALLBACK_URL=https://abc123.ngrok-free.app/api/mpesa/callback
```

Restart backend: `npm run dev`

#### Simulate a Test Payment
- Use sandbox test phone: `254708374149`
- In Daraja portal → APIs → Lipa Na M-Pesa Sandbox → Simulate → Submit

---

### Step E — Go Live (Production)

#### Prerequisites Checklist
- [ ] KYC verified for Till **4149288** with Safaricom
- [ ] Registered domain name (e.g., `naojaventures.com`)
- [ ] Valid SSL certificate on domain (HTTPS) — **required by Safaricom**
- [ ] Server running 24/7

#### Request Production Access
1. Daraja portal → **"Go Live"** on your app
2. Fill in:
   - Business Name: `Naoja Ventures`
   - Shortcode: `4149288`
   - Callback URL: `https://naojaventures.com/api/mpesa/callback`
3. Attach required business documents
4. Wait 1–5 business days for approval
5. You'll receive production credentials via email

#### Update Production `.env`
```dotenv
NODE_ENV=production
FRONTEND_URL=https://naojaventures.com
MPESA_BASE_URL=https://api.safaricom.co.ke
MPESA_CONSUMER_KEY=YOUR_PRODUCTION_CONSUMER_KEY
MPESA_CONSUMER_SECRET=YOUR_PRODUCTION_CONSUMER_SECRET
MPESA_SHORTCODE=4149288
MPESA_PASSKEY=YOUR_PRODUCTION_PASSKEY
MPESA_CALLBACK_URL=https://naojaventures.com/api/mpesa/callback
```

---

### How STK Push Works End-to-End

```
Customer clicks "Pay with M-Pesa"
          │
          ▼
Frontend → POST /api/mpesa/stkpush { phone, amount, orderId, orderRef }
          │
          ▼
Backend:
  1. Normalize phone → 254XXXXXXXXX
  2. Get OAuth token from Daraja
  3. Generate timestamp + password (Base64)
  4. POST to Daraja /mpesa/stkpush/v1/processrequest
  5. Save CheckoutRequestID to order in PostgreSQL
  6. Return { success: true, checkoutRequestId }
          │
          ▼
Safaricom → STK Push notification on customer's phone
          │
Customer enters M-Pesa PIN
          │
          ▼
Safaricom → POST https://naojaventures.com/api/mpesa/callback
          │
Backend:
  1. ResultCode === 0 → payment success
  2. Update order: payment_status='paid', mpesa_transaction_id=receipt
  3. Respond { ResultCode: 0, ResultDesc: "Accepted" } to Safaricom
          │
          ▼
Frontend polls /api/mpesa/order-status/:orderRef every 3 seconds
  → When paid → show "Order Confirmed" screen ✅
```

---

### Common M-Pesa Error Codes

| Code | Meaning | Action |
|---|---|---|
| `0` | ✅ Success | Payment confirmed |
| `1` | Insufficient funds | Customer should top up |
| `17` | Risk limits exceeded | Daily limit reached |
| `1032` | Cancelled by user | Customer pressed cancel |
| `1037` | Timeout / in progress | Wait and retry |
| `2001` | Wrong PIN | Customer entered wrong PIN |
| `400.002.05` | Invalid consumer key | Check MPESA_CONSUMER_KEY |

---

## 6. Admin Portal Guide

**URL:** `https://naojaventures.com/admin`

| Section | What You Can Do |
|---|---|
| **Dashboard** | View total orders, revenue, pending orders |
| **Orders** | View all orders, update order status, see M-Pesa receipts |
| **Products** | Add/edit/delete products, upload images, set featured |
| **Reviews** | Approve or delete customer product reviews |
| **Messages** | View customer contact form submissions |
| **Subscribers** | View newsletter subscriber list |

**Order Status Flow:**
`pending` → `confirmed` → `shipped` → `delivered`

**Payment Status Values:**
- `pending` — Waiting for M-Pesa payment
- `paid` — Payment confirmed by Safaricom
- `failed` — Payment failed or cancelled

---

## 7. Database Schema Reference

### PostgreSQL — `orders` Table (Key Fields)
```
order_ref                  → Unique order ID (e.g. "ORD-1696000000-XY3")
customer_name              → Customer's full name
customer_phone             → Phone number (also used for M-Pesa matching)
total_amount               → Order total in KES
status                     → pending/confirmed/shipped/delivered/cancelled
payment_status             → pending/paid/failed
mpesa_transaction_id       → M-Pesa receipt (e.g. "NKJ0000XYZ")
mpesa_checkout_request_id  → Daraja request ID (for callback matching)
items                      → JSON array of ordered products
```

### MongoDB — `products` Collection (Key Fields)
```
name            → Product name
slug            → URL identifier (e.g. "samsung-galaxy-a15")
price           → Price in KES
stockQuantity   → Available stock
category        → Reference to category
images          → Array of image URLs
status          → in_stock / low_stock / out_of_stock
isFeatured      → Show on homepage featured section
```

---

## 8. API Endpoint Reference

### Authentication
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/auth/login` | Admin login (15 attempts / 15min limit) |
| GET | `/api/auth/verify` | Verify JWT token |

### Products
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/products` | List products (`?category=&search=&page=`) |
| GET | `/api/products/:id` | Get single product |
| POST | `/api/products` | Create product (Admin) |
| PUT | `/api/products/:id` | Update product (Admin) |
| DELETE | `/api/products/:id` | Delete product (Admin) |

### Orders
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/orders` | Create new order |
| GET | `/api/orders` | List all orders (Admin) |
| PUT | `/api/orders/:id/status` | Update order status (Admin) |
| GET | `/api/orders/customer/:phone` | Get customer orders by phone |

### M-Pesa
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/mpesa/stkpush` | Initiate STK Push (6 requests/min limit) |
| POST | `/api/mpesa/callback` | Safaricom payment callback webhook |
| POST | `/api/mpesa/query` | Query STK status from Daraja |
| GET | `/api/mpesa/order-status/:orderRef` | Get order payment status |

### Health Check
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/health` | Server health check |

---

## 9. Production Deployment (VPS/Cloud Server)

Assumes **Ubuntu 22.04** VPS (DigitalOcean, Hetzner, AWS EC2, etc.)

### Server Preparation
```bash
# System update
sudo apt update && sudo apt upgrade -y

# Node.js 18
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs nginx certbot python3-certbot-nginx

# PM2
sudo npm install -g pm2

# MongoDB
sudo apt-get install -y gnupg
curl -fsSL https://www.mongodb.org/static/pgp/server-6.0.asc | sudo gpg -o /usr/share/keyrings/mongodb-server-6.0.gpg --dearmor
echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-6.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/6.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-6.0.list
sudo apt-get update && sudo apt-get install -y mongodb-org
sudo systemctl start mongod && sudo systemctl enable mongod

# PostgreSQL
sudo apt-get install -y postgresql postgresql-contrib
sudo systemctl start postgresql && sudo systemctl enable postgresql
```

#### PostgreSQL Setup
```bash
sudo -u postgres psql
```
```sql
CREATE DATABASE naoja_shop;
CREATE USER naoja_user WITH PASSWORD 'choose_a_strong_password';
GRANT ALL PRIVILEGES ON DATABASE naoja_shop TO naoja_user;
\q
```

---

### Deploy Backend
```bash
# Create directory and clone
sudo mkdir -p /var/www/naojaretailshop
sudo chown -R $USER:$USER /var/www/naojaretailshop
cd /var/www/naojaretailshop
git clone https://github.com/Apollo-otete/Naojaretailshop.git .

# Install and configure
cd backend
npm install --production
nano .env  # Fill in production environment variables

# Seed database (first time only)
node src/scripts/seed.js

# Start with PM2
pm2 start server.js --name "naoja-backend"
pm2 save
pm2 startup
# Run the command it outputs
```

---

### Deploy Frontend
```bash
cd /var/www/naojaretailshop/frontend
npm install
npm run build
# Production bundle is in: frontend/dist/
```

---

### Nginx + HTTPS Configuration
```bash
sudo nano /etc/nginx/sites-available/naoja
```

```nginx
server {
    listen 80;
    server_name naojaventures.com www.naojaventures.com;

    root /var/www/naojaretailshop/frontend/dist;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass         http://localhost:5000/api/;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection 'upgrade';
        proxy_set_header   Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 60s;
    }

    location /uploads/ {
        proxy_pass http://localhost:5000/uploads/;
        proxy_set_header Host $host;
        expires 30d;
    }

    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
    add_header X-XSS-Protection "1; mode=block";
}
```

```bash
# Enable site
sudo ln -s /etc/nginx/sites-available/naoja /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

# Install SSL (HTTPS) — required for M-Pesa
sudo certbot --nginx -d naojaventures.com -d www.naojaventures.com
```

---

### Go-Live Checklist

- [ ] Frontend loads at `https://naojaventures.com`
- [ ] Admin panel works at `https://naojaventures.com/admin`
- [ ] Health check OK: `https://naojaventures.com/api/health`
- [ ] Admin password changed from default `admin123`
- [ ] Products loading from MongoDB
- [ ] SSL certificate active (padlock in browser)
- [ ] M-Pesa callback URL registered in Daraja portal
- [ ] Test STK Push with KES 1 before launch
- [ ] PM2 auto-starts on reboot

---

## 10. Security Measures

| Category | Measure | Detail |
|---|---|---|
| **Auth** | Password hashing | bcrypt with cost factor 10 |
| **Auth** | JWT tokens | Expire after 24 hours |
| **Auth** | Rate limiting | 15 login attempts per 15 min per IP |
| **Payments** | M-Pesa rate limiting | 6 STK Push requests per minute per IP |
| **Payments** | Callback validation | Validates Safaricom payload structure |
| **Payments** | Dual match strategy | CheckoutRequestID + phone number fallback |
| **Database** | SQL parameterization | All queries use `$1, $2` — no SQL injection |
| **Network** | CORS protection | Only `FRONTEND_URL` allowed in production |
| **Network** | Payload limits | JSON body limited to 5MB |
| **Server** | HTTPS | Let's Encrypt SSL on all traffic |
| **Server** | Security headers | X-Frame-Options, XSS-Protection, etc. |
| **Config** | .env in .gitignore | Secrets never committed to Git |

---

## 11. Troubleshooting Guide

### Backend Won't Start

**`Cannot find module`** → Run `npm install` in the `backend/` directory  
**`Connection refused on port 5432`** → Run `docker-compose up -d` (PostgreSQL not running)  
**`MongooseServerSelectionError`** → Run `docker-compose up -d` (MongoDB not running)  

### M-Pesa Issues

**`Daraja OAuth failed`** → Check `MPESA_CONSUMER_KEY` / `MPESA_CONSUMER_SECRET` and `MPESA_BASE_URL`  
**STK Push not received** → Verify phone is `254XXXXXXXXX` format and M-Pesa is active  
**Callback not received** → Verify `MPESA_CALLBACK_URL` is public HTTPS; check `pm2 logs naoja-backend`  
**Order stuck at `pending`** → Use `/api/mpesa/query` to manually check payment status  
**`ResultCode: 1032`** → Customer cancelled — normal behavior, ask them to try again  

### Frontend Issues

**CORS errors** → Set `FRONTEND_URL` in `backend/.env` to exact frontend URL (no trailing slash)  
**Products not showing** → Check MongoDB is running; check `pm2 logs naoja-backend`  
**Admin login fails** → Verify `JWT_SECRET` is set; check admin user exists in DB  

---

## 12. Quick Reference Cheatsheet

### Start Development
```bash
cd backend && npm run dev          # Terminal 1 — Backend (http://localhost:5000)
cd frontend && npm run dev         # Terminal 2 — Frontend (http://localhost:5173)
cd backend && docker-compose up -d # Terminal 3 — Databases
```

### Production Management
```bash
pm2 logs naoja-backend --lines 100       # View logs
pm2 restart naoja-backend                # Restart after changes
cd backend && git pull && pm2 restart naoja-backend   # Deploy backend update
cd frontend && git pull && npm run build               # Deploy frontend update
curl https://naojaventures.com/api/health              # Health check
```

### Key URLs

| URL | Description |
|---|---|
| `https://naojaventures.com` | Customer storefront |
| `https://naojaventures.com/admin` | Admin login |
| `https://naojaventures.com/api/health` | API health check |
| `https://developer.safaricom.co.ke` | Daraja developer portal |

### M-Pesa Environment Variables

| Variable | Sandbox | Production |
|---|---|---|
| `MPESA_BASE_URL` | `https://sandbox.safaricom.co.ke` | `https://api.safaricom.co.ke` |
| `MPESA_SHORTCODE` | `174379` | `4149288` |
| `MPESA_CONSUMER_KEY` | Daraja sandbox app | Daraja production app |
| `MPESA_CONSUMER_SECRET` | Daraja sandbox app | Daraja production app |
| `MPESA_PASSKEY` | Daraja Lipa Na M-Pesa Sandbox page | Daraja Go Live credentials |
| `MPESA_CALLBACK_URL` | `https://your-ngrok.ngrok-free.app/api/mpesa/callback` | `https://naojaventures.com/api/mpesa/callback` |

---

*Documentation prepared for Naoja Ventures by Apollos Tech.*  
*For technical support, contact the development team.*
