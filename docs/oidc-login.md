# OIDC Login (Control Panel)

The control panel supports external identity provider login via OpenID Connect (OIDC). Platform operators configure providers (Google, Microsoft, or custom OIDC providers), and users see "Sign in with Google"-style buttons on the login page.

## How It Works

OIDC login does **not** create accounts. Users must already exist in the system. The flow is:

1. User creates an account (or is provisioned) with email/password
2. User logs in with email/password, goes to Profile, clicks "Connect Google Account"
3. This links their Google identity (`sub` claim) to their user account
4. From now on, the user can log in with either email/password or "Sign in with Google"

Identity binding uses the `sub` claim from the OIDC provider — not email matching. This is more secure and handles email changes correctly.

## Architecture

```
Login page                          Profile page (authenticated)
  ├── "Sign in with Google"           └── "Connect Google Account"
  │     ↓                                    ↓
  │   GET /auth/oidc/authorize         POST /api/v1/me/oidc-connections/authorize
  │   (state: mode=login, partner)     (state: mode=connect, user_id, partner)
  │     ↓                                    ↓
  │   → Google OAuth consent →         → Google OAuth consent →
  │     ↓                                    ↓
  │   GET /auth/oidc/callback          GET /auth/oidc/callback
  │   (lookup user by sub)             (store connection for user)
  │     ↓                                    ↓
  │   Redirect /?token=<jwt>           Redirect /profile?oidc=connected
```

## Configuration

Set environment variables to enable OIDC providers:

```bash
# Comma-separated list of provider IDs to enable
OIDC_PROVIDERS=google,microsoft

# Google (well-known URLs hardcoded)
OIDC_GOOGLE_CLIENT_ID=your-google-client-id
OIDC_GOOGLE_CLIENT_SECRET=your-google-client-secret

# Microsoft (well-known URLs hardcoded)
OIDC_MICROSOFT_CLIENT_ID=your-microsoft-client-id
OIDC_MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret

# Custom provider (requires issuer URL for endpoint discovery)
OIDC_CUSTOM_CLIENT_ID=your-client-id
OIDC_CUSTOM_CLIENT_SECRET=your-client-secret
OIDC_CUSTOM_ISSUER_URL=https://login.example.com
OIDC_CUSTOM_NAME=My Company SSO
```

### Callback URL Registration

Each partner's callback URL must be registered with the OAuth provider:

```
https://<partner-hostname>/auth/oidc/callback
```

For example, if the partner hostname is `home.acme-hosting.com`, register:
```
https://home.acme-hosting.com/auth/oidc/callback
```

### Well-Known Providers

Google and Microsoft have hardcoded endpoint URLs:

| Provider | Auth URL | Token URL | UserInfo URL |
|---|---|---|---|
| Google | `accounts.google.com/o/oauth2/v2/auth` | `oauth2.googleapis.com/token` | `openidconnect.googleapis.com/v1/userinfo` |
| Microsoft | `login.microsoftonline.com/common/oauth2/v2.0/authorize` | `login.microsoftonline.com/common/oauth2/v2.0/token` | `graph.microsoft.com/oidc/userinfo` |

Custom providers derive endpoints from the issuer URL (`/authorize`, `/token`, `/userinfo`).

## Database

The `user_oidc_connections` table links users to provider identities:

```sql
CREATE TABLE user_oidc_connections (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    partner_id TEXT NOT NULL REFERENCES partners(id),
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    email      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider),
    UNIQUE(partner_id, provider, subject)
);
```

- `UNIQUE(user_id, provider)` — a user can connect one account per provider
- `UNIQUE(partner_id, provider, subject)` — a provider identity can only be linked to one user per partner

## API Endpoints

### Public (partner middleware, no JWT)

| Method | Path | Description |
|---|---|---|
| GET | `/auth/oidc/providers` | List enabled providers (`{items: [{id, name}]}`) |
| GET | `/auth/oidc/authorize?provider=google` | Redirect to provider's OAuth consent page (login mode) |
| GET | `/auth/oidc/callback?code=...&state=...` | Handle OAuth callback (login or connect mode) |

### Authenticated (JWT required)

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/me/oidc-connections` | List user's connected providers |
| POST | `/api/v1/me/oidc-connections/authorize?provider=google` | Get redirect URL for connecting a provider |
| DELETE | `/api/v1/me/oidc-connections/{provider}` | Disconnect a provider |

## Security

- **State parameter:** HMAC-signed JSON (reuses JWT secret) with nonce, expiry (10 min), mode, partner ID, and user ID. Prevents CSRF attacks.
- **Subject-based binding:** Uses `sub` claim, not email. This prevents account takeover via email changes at the provider.
- **No account creation:** OIDC login only works for existing users who have explicitly connected their provider identity.
- **Partner scoping:** A provider identity is scoped to a partner — the same Google account can be linked to different users on different partner instances.

## Frontend

### Login Page

- Fetches available providers from `GET /auth/oidc/providers` on mount
- Shows provider buttons above the email/password form with an "or" divider
- Buttons navigate to `/auth/oidc/authorize?provider=<id>` (full page navigation)
- Handles `?token=` query param from callback (stores JWT, redirects to dashboard)
- Handles `?oidc_error=no_account` (shows error message directing user to connect account first)

### Profile Page

- Shows "Connected Accounts" section when OIDC providers are enabled
- Lists each provider with connect/disconnect buttons
- "Connect" button calls `POST /api/v1/me/oidc-connections/authorize`, navigates to returned URL
- "Disconnect" button calls `DELETE /api/v1/me/oidc-connections/{provider}`
- Handles `?oidc=connected` query param (shows success toast)
