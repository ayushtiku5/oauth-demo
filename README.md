# OAuth 2.0 + Google Calendar — Go Prototype

## What this shows

```
Browser          Go Server              Google OAuth         Google Calendar API
   |                  |                      |                      |
   |  GET /auth/google|                      |                      |
   |----------------->|                      |                      |
   |                  | redirect to accounts.|                      |
   |                  | google.com/o/oauth2/ |                      |
   |<-----------------|  auth?client_id=...  |                      |
   |                                         |                      |
   |  user logs in & consents                |                      |
   |---------------------------------------->|                      |
   |                                         |                      |
   |  redirect to /auth/google/callback?code=|                      |
   |<----------------------------------------|                      |
   |                  |                      |                      |
   |  GET /callback   |                      |                      |
   |----------------->|                      |                      |
   |                  | POST /token (code)   |                      |
   |                  |--------------------->|                      |
   |                  | { access_token, ... }|                      |
   |                  |<---------------------|                      |
   |                  |                                             |
   |  GET /calendar   |  GET /calendar/v3/... Bearer <access_token>|
   |----------------->|-------------------------------------------->|
   |                  |           [ { event }, ... ]                |
   |  JSON events     |<--------------------------------------------|
   |<-----------------|                                             |
```

## Setup — Google Cloud Console

1. Go to [console.cloud.google.com](https://console.cloud.google.com) → **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth 2.0 Client ID**
3. Application type: **Web application**
4. Add `http://localhost:8080/auth/google/callback` to **Authorized redirect URIs**
5. Copy the **Client ID** and **Client Secret**
6. Enable the **Google Calendar API** under **APIs & Services** → **Enabled APIs**

## Run

```bash
export GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="your-client-secret"
go run main.go
```

Then open http://localhost:8080 in your browser.

## Endpoints

| Route | Purpose |
|---|---|
| `GET /` | Home page with login link |
| `GET /auth/google` | Redirects browser to Google's authorization page |
| `GET /auth/google/callback` | Receives `code` from Google, exchanges it for a token |
| `GET /calendar` | Uses the stored token to fetch the next 10 calendar events |

## Key concepts illustrated

- **Authorization Code flow** — the browser never sees the client secret; the server exchanges the code server-side
- **State parameter** — random nonce stored in a cookie, verified on callback to prevent CSRF
- **Token exchange** — `oauthConfig.Exchange(ctx, code)` turns a one-time code into a reusable access token
- **Automatic token refresh** — `oauthConfig.Client(ctx, token)` returns an `http.Client` that refreshes the token transparently when it expires
- **`AccessTypeOffline`** — tells Google to include a refresh token so the user doesn't have to re-authorize
