# Naoja Retail Shop

An e-commerce application for browsing electronics, managing a shopping cart, placing orders, and administering store operations.

## Features

- Product catalog, categories, and search
- Shopping cart and checkout
- M-Pesa payment integration (requires backend configuration)
- Order tracking
- Admin tools for products, orders, reviews, and analytics

## Technology

- Frontend: React, Vite, Tailwind CSS
- Backend: Node.js and Express
- Data: MongoDB and PostgreSQL
- Payments: Safaricom Daraja API

## Local Development

### Requirements

- Node.js 18 or later and npm
- Docker Compose, or running MongoDB and PostgreSQL instances

### Setup

From the project root, install the root, backend, and frontend dependencies:

```bash
npm install
npm run install:all
```

Start the local database containers if you are using Docker:

```bash
docker compose -f backend/docker-compose.yml up -d
```

Create the backend environment file and configure it for your local databases:

```bash
cp backend/.env.example backend/.env
```

Start the frontend and backend together:

```bash
npm run dev
```

The frontend runs on the Vite development server, normally `http://localhost:5173`. The backend listens on port `5000`. If the frontend port is already in use, Vite selects another available port and prints it in the terminal.

Build the frontend for production:

```bash
npm run build
```

## Application Structure

- `frontend/src/pages/` contains the main storefront and admin pages.
- `frontend/src/components/` contains shared interface components.
- `frontend/src/lib/api.jsx` contains the frontend API client.
- `backend/src/routes/` contains Express API routes.
- `backend/src/models/` contains MongoDB models.
- `backend/src/config/` contains database configuration and initialization.

During development, Vite proxies `/api` requests to the backend on port `5000`.

## Configuration and Security

- Keep real credentials in `backend/.env`; do not commit environment files or secrets.
- `backend/.env.example` contains placeholders. Replace them with values for your environment.
- Use unique, strong database passwords and a strong `JWT_SECRET` outside local development.
- Treat all `VITE_*` values as public: frontend environment values are included in browser code. Never put private credentials there.
- The Docker Compose database setup is for local development only. Do not expose its services publicly or reuse local credentials in production.
- For deployment, use HTTPS, restrict allowed frontend origins, and configure a secure public HTTPS callback URL for M-Pesa.

## Project Scripts

- `npm run dev` starts the backend and frontend development servers.
- `npm run dev:backend` starts only the backend.
- `npm run dev:frontend` starts only the frontend.
- `npm run build` creates the frontend production build.
