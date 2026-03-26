---
name: configure
description: First-time setup wizard for the GWS connector. Checks configuration, guides through adding accounts.
command: gws:configure
---

# GWS Configure

Help the user set up the GWS connector plugin for the first time.

## Steps

1. Check if `GWS_GOOGLE_CLIENT_ID` and `GWS_GOOGLE_CLIENT_SECRET` environment variables are set. If not, guide the user:
   - Go to https://console.cloud.google.com/apis/credentials
   - Create a new project (or use existing)
   - Enable the Gmail API, Google Calendar API, and Google Drive API
   - Create an OAuth 2.0 Client ID (Application type: Desktop app)
   - Copy the Client ID and Client Secret
   - Set them as environment variables in their shell profile or Claude Code settings

2. Once credentials are configured, check if any accounts are connected by calling `gws.accounts.list`.

3. If no accounts exist, offer to add the first one using `gws.accounts.add` with a label the user chooses.

4. After the first account is added, offer to add more accounts.

5. If existing built-in Claude Google Calendar or Gmail connectors are detected, explain:
   - The GWS connector plugin provides multi-account support and Drive access
   - Both can coexist — tool names don't conflict
   - The user can use either, or disconnect the built-in ones for a simpler experience
