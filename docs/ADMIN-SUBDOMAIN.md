# Admin subdomain (separate origin, separate build artifact)

## Local test (`admin.naoja.local`)

Storefront stays on `http://localhost:5173` (Vite dev) or `npm run preview`.

Admin console (separate build) on `http://admin.naoja.local:5174/admin/`.

### 1) Hostname -> localhost (requires sudo)
```sh
echo "127.0.0.1 admin.naoja.local" | sudo tee -a /etc/hosts
```

### 2) Build both
```sh
cd frontend
npm install
npm run build          # dist/
npm run build:admin    # dist-admin/  (base: /admin/)
```

### 3) Serve admin console on the admin hostname
```sh
cd frontend
npm run preview:admin
# -> http://admin.naoja.local:5174/admin/
```

### 4) Backend (required for login/session/CSRF)
`backend-go/.env` must include the admin origin:
```
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5174,http://admin.naoja.local:5174,http://admin.naoja.local
```
Restart the Go API.

### 5) Access
- Admin login: `http://admin.naoja.local:5174/admin/login`
- Admin console: `http://admin.naoja.local:5174/admin/`

## Production (same VPS nginx serves static files)
Serve storefront and admin as **separate origins**:

- Storefront build (`dist/`) at e.g. `naoja.local` / production domain.
- Admin build (`dist-admin/`) at `admin.naoja.local` / production `admin.<domain>`.

Use the server-block template in `infra/nginx/admin-subdomain.conf` (or inline equivalents) and point:
- `root` -> respective build directory
- `try_files` -> respective entry HTML
- `/api/` -> `127.0.0.1:8080`

For production `.env`:
```
ALLOWED_ORIGINS=https://<storefront-domain>,https://admin.<domain>
```
Also terminate TLS at nginx and set `Secure` (the cookie helper already uses `IsProduction()`).

## Important
- The admin console remains an isolated build (`dist-admin/`); the storefront does **not** ship admin pages.
- Backend remains the authorization boundary: admin routes require an admin session/roles. UI paths do not grant access.
- Restart the Go API after changing `ALLOWED_ORIGINS`.
