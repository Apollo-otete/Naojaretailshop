# Naoja Retail Shop

An e-commerce application for browsing electronics, managing a shopping cart, placing orders, and administering store operations. The backend is **`backend-go/`** (Go + Gin + PostgreSQL), serving the `/api/v1` contract and a legacy `/api/*` compatibility layer for the existing frontend.

## Features

- Product catalog, categories, and search
- Shopping cart and checkout
- M-Pesa payment integration (Daraja adapter not yet wired; backend configuration required)
- Order tracking
- Admin tools for products, orders, reviews, and analytics

## Technology

- Frontend: React, Vite, Tailwind CSS
- Backend: Go (Gin) — `backend-go/`
- Data: PostgreSQL (migrations in `backend-go/migrations/sql`)
- Payments: Safaricom Daraja API (planned)

## Local Development

### Requirements

- Node.js 18+ and npm (frontend)
- Go (see `backend-go/go.mod`) and Docker (PostgreSQL for the API)

### Setup

Install frontend dependencies:

```bash
npm install
npm run install:all
```

Start PostgreSQL and run the API (see `backend-go/README.md` for full details):

```bash
cd backend-go
make db-up                     # PostgreSQL (+ naoja_test)
cp -n .env.example .env
make migrate-up
make run                       # API on :8080
```

In another terminal, start the frontend (proxies `/api` → `:8080`):

```bash
npm run dev:frontend           # or: npm --prefix frontend run dev
```

Or run both together from the repo root (requires `make`/Go on PATH):

```bash
npm run dev
```

The frontend runs on Vite (normally `http://localhost:5173`). The Go API listens on `:8080` by default.

Build the frontend for production:

```bash
npm run build
```

## Application Structure

- `frontend/src/pages/` contains the main storefront and admin pages.
- `frontend/src/components/` contains shared interface components.
- `frontend/src/lib/api.jsx` contains the frontend API client.
- `backend-go/cmd/api/` is the Go API entrypoint; `backend-go/internal/` holds domain, ports, adapters, and HTTP handlers.
- `backend-go/migrations/sql/` contains reversible PostgreSQL migrations.
- `contracts/openapi.yaml` is the `/api/v1` contract.

During development, Vite proxies `/api` requests to the Go backend on port `8080`. The backend serves `/api/v1` (contract) and `/api/*` (legacy compatibility for the existing frontend).

## Configuration and Security

- Keep real credentials in `backend-go/.env`; do not commit environment files or secrets. `backend-go/.env.example` contains placeholders.
- Use unique, strong database passwords and strong secrets outside local development.
- Treat all `VITE_*` values as public (included in browser code). Never put private credentials there.
- The Docker Compose database setup is for local development only. Do not expose its services publicly or reuse local credentials in production.
- For deployment, use HTTPS, restrict allowed frontend origins, and configure a secure public HTTPS callback URL for M-Pesa.

## Project Scripts

- `npm run dev` starts the Go backend (`make -C backend-go run`) and frontend dev servers together.
- `npm run dev:backend` starts only the Go backend.
- `npm run dev:frontend` starts only the frontend.
- `npm run build` creates the frontend production build.
- `cd backend-go && make check` runs the Go backend's fmt/vet/unit-test gate.
