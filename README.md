# Insighta Labs+ — Backend

The core API server for the Insighta Labs+ platform. Built on top of the Profile Intelligence System, extended with authentication, role-based access control, and multi-interface support.

---

## System Architecture

The platform consists of three independent repositories that share a single backend:

```
┌─────────────────┐         ┌─────────────────────────────┐
│   Browser       │────────▶│   insighta-web (port 3000)  │
│   (Web Portal)  │◀────────│   Go SSR server             │
└─────────────────┘         └────────────┬────────────────┘
                                         │ HTTP + JWT
┌─────────────────┐                      ▼
│   Terminal      │         ┌─────────────────────────────┐
│   (CLI)         │────────▶│   insighta-api (port 8080)  │
└─────────────────┘         │   Go REST API + PostgreSQL  │
                            └─────────────────────────────┘
```

Both the CLI and the web portal are clients of the backend API. The backend is the single source of truth — all data, authentication, and business logic lives here.

---

## Authentication Flow

This system uses GitHub OAuth 2.0 with PKCE (Proof Key for Code Exchange).

### Browser Flow
```
1. User visits /auth/github
2. Backend generates state + code_verifier + code_challenge
3. Backend redirects user to GitHub OAuth page
4. User approves on GitHub
5. GitHub redirects to GET /auth/github/callback with a code
6. Backend exchanges code + code_verifier with GitHub for an access token
7. Backend calls GitHub /user API to get user info
8. Backend upserts user in database
9. Backend issues JWT access token + refresh token
10. Tokens returned as JSON (web portal stores them as HTTP-only cookies)
```

### CLI Flow
```
1. CLI generates state + code_verifier + code_challenge locally
2. CLI starts a temporary local server on port 9999
3. CLI opens GitHub OAuth page in browser
4. GitHub redirects to localhost:9999/callback
5. CLI captures code + validates state
6. CLI sends code + code_verifier to POST /auth/github/callback
7. Backend exchanges with GitHub, upserts user, returns tokens
8. CLI saves tokens to ~/.insighta/credentials.json
```

### PKCE Explained
PKCE prevents code interception attacks. The CLI generates a random `code_verifier`, hashes it to produce a `code_challenge`, and sends the challenge to GitHub upfront. When exchanging the code, it sends the original verifier. GitHub hashes it and confirms it matches — proving the exchange comes from the same client that started the flow.

---

## Token Handling

| Token | Type | Expiry | Storage |
|-------|------|--------|---------|
| Access token | JWT (HS256) | 3 minutes | Client-side (cookie or file) |
| Refresh token | Random string | 5 minutes | Database + client |

**Refresh token rotation:** Every call to `POST /auth/refresh` invalidates the old refresh token and issues a new pair. One-time use only.

**JWT claims:** `id` (user UUID), `role` (admin/analyst), `exp` (expiry timestamp)

---

## Role Enforcement

Two roles exist in the system:

| Role | Permissions |
|------|-------------|
| `admin` | Full access — create profiles, delete profiles, read, search, export |
| `analyst` | Read only — get, list, search, export profiles |

Default role on first login: `analyst`

### How it works
Role enforcement uses two middleware layers applied at the route level:

1. **AuthMiddleware** — validates the JWT, extracts `id` and `role`, injects them into the request context
2. **RBACMiddleware("admin")** — reads role from context, rejects with 403 if the user is not an admin

```
Request → AuthMiddleware → RBACMiddleware("admin") → Handler
```

Routes requiring admin: `POST /api/profiles`, `DELETE /api/profiles/{id}`

All other `/api/*` routes require authentication but not a specific role.

---

## Natural Language Parsing

The `GET /api/profiles/search?q=` endpoint accepts plain English queries and converts them to structured SQL filters.

**How it works:**
1. Query string is lowercased and split into tokens
2. Keywords are matched against a whitelist — no user input is ever interpolated into SQL
3. Matched values are passed as `$1`, `$2` parameterized query arguments

**Supported patterns:**

| Query | Parsed as |
|-------|-----------|
| `young males from nigeria` | gender=male, country=NG, age_group=young adult |
| `females above 30` | gender=female, min_age=30 |
| `adult males from kenya` | gender=male, age_group=adult, country=KE |
| `seniors` | age_group=senior |
| `teenagers from us` | age_group=teenager, country=US |

Age group mapping: child (0-12), teenager (13-17), young/young adult (18-30), adult (18-65), senior (65+)

---

## Running Locally

**Prerequisites:** Go 1.26+, PostgreSQL

```bash
git clone <repo>
cd insighta-api

# Create .env file
cp .env.example .env
# Fill in GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, JWT_SECRET

# Tables are created automatically on startup
go run main.go

# Seed with sample data
go run cmd/seed/main.go
```

## Environment Variables

```env
DATABASE_URL=postgres://user:password@localhost:5432/profiles_db
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
JWT_SECRET=
REDIRECT_URI=http://localhost:8080/auth/github/callback
PORT=8080
```

---

## API Reference

### Auth Endpoints
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/auth/github` | Redirect to GitHub OAuth |
| GET | `/auth/github/callback` | Browser OAuth callback |
| POST | `/auth/github/callback` | CLI OAuth callback |
| POST | `/auth/refresh` | Refresh tokens |
| POST | `/auth/logout` | Invalidate refresh token |

### Profile Endpoints
All require `Authorization: Bearer <token>` and `X-API-Version: 1`

| Method | Endpoint | Role | Description |
|--------|----------|------|-------------|
| POST | `/api/profiles` | admin | Create profile |
| GET | `/api/profiles` | any | List with filters + pagination |
| GET | `/api/profiles/search` | any | Natural language search |
| GET | `/api/profiles/export` | any | Export CSV |
| GET | `/api/profiles/{id}` | any | Get by ID |
| DELETE | `/api/profiles/{id}` | admin | Delete profile |

### Pagination Response
```json
{
  "status": "success",
  "page": 1,
  "limit": 10,
  "total": 100,
  "total_pages": 10,
  "links": {
    "self": "/api/profiles?page=1&limit=10",
    "next": "/api/profiles?page=2&limit=10",
    "prev": null
  },
  "data": []
}
```

### Rate Limiting
| Scope | Limit |
|-------|-------|
| `/auth/*` | 10 requests/minute |
| `/api/*` | 60 requests/minute |

---

## Deployment (EC2)

```bash
# SSH into instance
ssh -i hng-key.pem ubuntu@YOUR_IP

# Pull and rebuild
git pull
go build -o server .

# Create/update .env with production values
nano .env

# Restart server
pkill server
nohup ./server > server.log 2>&1 &

# View logs
tail -f server.log
```
