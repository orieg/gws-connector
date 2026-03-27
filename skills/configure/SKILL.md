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
>    - Name it something like "GWS Connector"
>    - Click "Create"
>
> **3. Enable the required APIs** — go to each link and click "Enable":
>    - Gmail API: https://console.cloud.google.com/apis/library/gmail.googleapis.com
>    - Google Calendar API: https://console.cloud.google.com/apis/library/calendar-json.googleapis.com
>    - Google Drive API: https://console.cloud.google.com/apis/library/drive.googleapis.com
>
> **4. Configure the OAuth consent screen:**
>    - Go to: https://console.cloud.google.com/auth/consent
>    - Choose "External" (unless you have a Google Workspace org and want "Internal")
>    - Fill in the app name (e.g., "Claude GWS") and your email for support contact
>    - Click "Save"
>
> **5. Add scopes** — go to the Data Access tab:
>    - Go to: https://console.cloud.google.com/auth/scopes
>    - Click "Add or Remove Scopes"
>    - In the panel that opens, paste these scopes into the "Manually add scopes" box at the bottom (one per line):
>      - `https://www.googleapis.com/auth/gmail.modify`
>      - `https://www.googleapis.com/auth/calendar`
>      - `https://www.googleapis.com/auth/drive`
>      - `https://www.googleapis.com/auth/userinfo.email`
>      - `https://www.googleapis.com/auth/userinfo.profile`
>    - Click "Update" to confirm, then "Save"
>
> **6. Add test users:**
>    - Go to: https://console.cloud.google.com/auth/audience
>    - Add each Google email address you plan to connect
>
> **7. Create OAuth credentials:**
>    - Go to: https://console.cloud.google.com/auth/clients
>    - Click "+ Create Client" → "OAuth client ID"
>    - Application type: **Desktop app**
>    - Name it (e.g., "Claude GWS Desktop")
>    - Click "Create"
>    - **Download the JSON file** (click the download icon next to your new client)

Ask the user to share or point to the downloaded JSON file, or provide the Client ID and Client Secret directly.

### 2b. Read credentials from JSON file

If the user provides a path to the downloaded `client_secret_*.json` file:

1. Read the file to extract `client_id` and `client_secret` from the `installed` object
2. The JSON structure looks like: `{"installed": {"client_id": "...", "client_secret": "..."}}`
3. Use these values when calling `gws.accounts.add`

If the user provides Client ID and Client Secret directly as text, use those.

**Important:** Do NOT suggest setting environment variables. Credentials are stored per-account in the OS keychain — no env vars needed.

## Step 3: Connect accounts

Once credentials are available, help the user add their first account:

1. Ask what label they want (e.g., "personal", "work", "client-name")
2. Call `gws.accounts.add` with the label, clientId, and clientSecret
3. This opens a browser — tell the user to authorize access
4. Confirm success — the client secret is now stored in the OS keychain

Then ask: "Would you like to connect another account? Each account can use different OAuth credentials if it belongs to a different organization."

## Step 4: Multiple organizations

If the user wants to connect accounts from different Google Workspace organizations:

- Each org needs its own GCP project with its own OAuth credentials
- Repeat Steps 2-3 for each org's GCP project
- Each account stores its own credentials independently in the keychain

## Step 5: Coexistence note

If the user already has Claude's built-in Google Calendar or Gmail connectors:

> You also have Claude's built-in Gmail/Calendar connectors. Both work side by side — there are no tool name conflicts. The GWS connector adds multi-account support and Google Drive. You can keep both or disconnect the built-in ones in Claude settings for a simpler experience.

## Important notes

- For Google Workspace (business) accounts, the Workspace admin may need to approve the OAuth app
- The "External" consent screen in testing mode allows up to 100 test users — add each Google email you plan to connect
- To publish the app (remove the "unverified" warning), you'd need to submit for Google verification — not needed for personal/testing use
- Client secrets are stored in the OS keychain (macOS Keychain, GNOME Keyring) — never in plain text config files
