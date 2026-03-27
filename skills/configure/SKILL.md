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
>    - **Download the JSON file** (click the download button)

Ask the user where they saved the downloaded `client_secret_*.json` file.

### 2b. Read credentials from the JSON file

Once the user provides the path to the downloaded JSON file:

1. Read the file using the Read tool
2. Extract `client_id` and `client_secret` from the `installed` object in the JSON
3. Store these values — you will pass them as `clientId` and `clientSecret` when adding accounts

The JSON file has this structure:
```json
{
  "installed": {
    "client_id": "...",
    "client_secret": "..."
  }
}
```

**Do NOT ask the user to set environment variables.** Credentials are passed directly per-account. This supports multi-account setups where different accounts use different GCP projects.

## Step 3: Connect accounts

Help the user add their first account:

1. Ask what label they want (e.g., "personal", "work", "client-name")
2. Call `gws.accounts.add` with the label AND the `clientId` and `clientSecret` extracted from the JSON file
3. This opens a browser — tell the user to authorize access
4. Confirm success

Then ask: "Would you like to connect another account? If it belongs to a different organization, you'll need a separate GCP project and credentials JSON for that org."

## Step 4: Coexistence note

If the user already has Claude's built-in Google Calendar or Gmail connectors:

> You also have Claude's built-in Gmail/Calendar connectors. Both work side by side — there are no tool name conflicts. The GWS connector adds multi-account support and Google Drive. You can keep both or disconnect the built-in ones in Claude settings for a simpler experience.

## Important notes

- For Google Workspace (business) accounts, the Workspace admin may need to approve the OAuth app
- The "External" consent screen in testing mode allows up to 100 test users — add each Google email you plan to connect
- To publish the app (remove the "unverified" warning), you'd need to submit for Google verification — not needed for personal/testing use
