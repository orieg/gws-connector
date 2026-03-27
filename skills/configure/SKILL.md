---
name: configure
description: Step-by-step setup wizard for the GWS connector. Walks through creating a GCP project, getting OAuth credentials, and connecting accounts.
---

# GWS Configure

Walk the user through the complete setup of the GWS connector plugin, step by step.

## Step 1: Check current state

Call `gws.accounts.list` to see if any accounts are already connected.

If accounts exist, show them and ask if the user wants to add another account or reconfigure.

## Step 2: Google Cloud Project setup

If no credentials are configured (the tool returns an error about missing credentials), guide the user through creating a GCP project:

### 2a. Create or select a GCP project

Tell the user:

> To connect Google accounts, you need OAuth credentials from a Google Cloud project. Here's how to set them up (one-time, ~5 minutes):
>
> **1. Go to the Google Cloud Console:**
>    https://console.cloud.google.com/
>
> **2. Create a new project** (or select an existing one):
>    - Click the project dropdown at the top → "New Project"
>    - Name it something like "Claude GWS Connector"
>    - Click "Create"
>
> **3. Enable the required APIs** — go to each link and click "Enable":
>    - Gmail API: https://console.cloud.google.com/apis/library/gmail.googleapis.com
>    - Google Calendar API: https://console.cloud.google.com/apis/library/calendar-json.googleapis.com
>    - Google Drive API: https://console.cloud.google.com/apis/library/drive.googleapis.com
>
> **4. Configure the OAuth consent screen:**
>    - Go to: https://console.cloud.google.com/auth/overview
>    - Under **Branding**, set the app name (e.g., "Claude GWS") and your email
>    - Under **Audience**, choose "External" (or "Internal" for Google Workspace orgs)
>    - Under **Audience → Test users**, add your Google email address(es)
>
> **5. Add API scopes:**
>    - Go to **Data Access** in the left sidebar (https://console.cloud.google.com/auth/scopes)
>    - Click "Add or remove scopes"
>    - Add these 5 scopes (paste each into the "Manually add scopes" box):
>      - `https://www.googleapis.com/auth/gmail.modify`
>      - `https://www.googleapis.com/auth/calendar`
>      - `https://www.googleapis.com/auth/drive`
>      - `https://www.googleapis.com/auth/userinfo.email`
>      - `https://www.googleapis.com/auth/userinfo.profile`
>    - Click "Update", then "Save"
>
> **6. Create OAuth credentials:**
>    - Go to **Clients** in the left sidebar (https://console.cloud.google.com/apis/credentials)
>    - Click "+ Create Client" → "OAuth client ID"
>    - Application type: **Desktop app**
>    - Name it (e.g., "Claude GWS Desktop")
>    - Click "Create"
>    - **Copy the Client ID and Client Secret** that appear

Ask the user to provide the Client ID and Client Secret once they have them.

### 2b. Configure credentials

Once the user provides credentials, explain the two ways to set them:

**Option A — Environment variables (recommended for a single GCP project):**

```bash
# Add to your shell profile (~/.zshrc, ~/.bashrc, etc.)
export GWS_GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GWS_GOOGLE_CLIENT_SECRET="your-client-secret"
```

Then restart Claude Code for the env vars to take effect.

**Option B — Per-account credentials (for different organizations):**

If different accounts belong to different Google Workspace organizations, each org may need its own GCP project. In this case, pass `clientId` and `clientSecret` directly when adding each account:

```
gws.accounts.add(label: "work", clientId: "work-client-id", clientSecret: "work-secret")
gws.accounts.add(label: "personal", clientId: "personal-client-id", clientSecret: "personal-secret")
```

This way each account uses its own org's OAuth app.

## Step 3: Connect accounts

Once credentials are configured, help the user add their first account:

1. Ask what label they want (e.g., "personal", "work", "client-name")
2. Call `gws.accounts.add` with the label (and per-account credentials if provided)
3. This opens a browser — tell the user to authorize access
4. Confirm success

Then ask: "Would you like to connect another account? Each account can use different OAuth credentials if it belongs to a different organization."

## Step 4: Coexistence note

If the user already has Claude's built-in Google Calendar or Gmail connectors:

> You also have Claude's built-in Gmail/Calendar connectors. Both work side by side — there are no tool name conflicts. The GWS connector adds multi-account support and Google Drive. You can keep both or disconnect the built-in ones in Claude settings for a simpler experience.

## Important notes

- For Google Workspace (business) accounts, the Workspace admin may need to approve the OAuth app
- The "External" consent screen in testing mode allows up to 100 test users — add each Google email you plan to connect
- To publish the app (remove the "unverified" warning), you'd need to submit for Google verification — not needed for personal/testing use
