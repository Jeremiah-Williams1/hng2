# Insighta Labs+ — Backend

The backend for the Insighta Labs+ platform. Built on top of the Profiles Intelligence System (Stage 2/3), now extended with authentication, role-based access control, and multi-interface support.

## System Architecture

Three repositories make up the full platform:
- **Backend** (this repo) — REST API, authentication, database
- **CLI** — `insighta-cli` — command line interface
- **Web Portal** — browser-based interface

Both the CLI and web portal talk to this backend.

## Authentication Flow

This backend uses GitHub OAuth with PKCE.

**Browser flow:**
1. Client hits `GET /auth/github`
2. Backend redirects to GitHub OAuth page
3. User approves on GitHub
4. GitHub redirects to `GET /auth/github/callback`
5. Backend exchanges code, upserts user, returns access + refresh tokens

**CLI flow:**
1. CLI starts a local server on port 9999
2. CLI opens GitHub OAuth page in browser
3. GitHub redirects to `localhost:9999/callback`
4. CLI catches the code, sends it to `POST /auth/github/callback` with the PKCE verifier
5. Backend exchanges code, returns access + refresh tokens
6. CLI saves tokens to `~/.insighta/credentials.json`

## Token Handling

| Token | Expiry | Storage |
|-------|--------|---------|
| Access token (JWT) | 3 minutes | Client-side |
| Refresh token | 5 minutes | Database + client |

Refresh tokens are single-use. Each refresh issues a new pair and invalidates the old refresh token.

## Roles

| Role | Permissions |
|------|-------------|
| `admin` | Full access — create, delete, read, search profiles |
| `analyst` | Read only — get, list, search profiles |

Default role on signup: `analyst`

## Running Locally

1. Clone the repo
2. Create a GitHub OAuth App:
   - Homepage URL: `http://localhost:8080`
   - Callback URL: `http://localhost:8080/auth/github/callback`
3. Copy `.env.example` to `.env` and fill in values
4. Set up PostgreSQL — tables are created automatically on startup
5. Run the seed script: `go run cmd/seed/main.go`
6. Start the server: `go run main.go`

## Environment Variables

```env
DATABASE_URL=postgres://user:password@localhost:5432/profiles_db
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
JWT_SECRET=
REDIRECT_URI=http://localhost:8080/auth/github/callback
PORT=8080
```

## Endpoints

### Auth
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/auth/github` | Redirect to GitHub OAuth |
| GET | `/auth/github/callback` | GitHub OAuth callback (browser) |
| POST | `/auth/github/callback` | GitHub OAuth callback (CLI) |
| POST | `/auth/refresh` | Refresh access token |
| POST | `/auth/logout` | Invalidate refresh token |

### Profiles
All profile endpoints require `Authorization: Bearer <token>` and `X-API-Version: 1` headers.

| Method | Endpoint | Role | Description |
|--------|----------|------|-------------|
| POST | `/api/profiles` | admin | Create a profile |
| GET | `/api/profiles` | any | List profiles with filters |
| GET | `/api/profiles/search` | any | Natural language search |
| GET | `/api/profiles/export` | any | Export profiles as CSV |
| GET | `/api/profiles/{id}` | any | Get profile by ID |
| DELETE | `/api/profiles/{id}` | admin | Delete a profile |

### Query Parameters (GET /api/profiles)
`gender`, `country_id`, `age_group`, `min_age`, `max_age`, `min_gender_probability`, `min_country_probability`, `sort_by`, `order`, `page`, `limit`

### Pagination Response Shape
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

## Rate Limiting
| Scope | Limit |
|-------|-------|
| `/auth/*` | 10 requests/minute |
| `/api/*` | 60 requests/minute per user |

## Natural Language Search Examples
- `?q=young males from nigeria`
- `?q=females above 30`
- `?q=adult males from kenya`

## Redeployment
```bash
git pull && go build -o server . && pkill server && nohup ./server > server.log 2>&1 &
```