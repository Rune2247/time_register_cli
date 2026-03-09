# Google Cloud Setup Guide for TimeReg

Step-by-step guide with exact clicks for setting up Google API access.

## Step 1: Create a Google Cloud Project

1. Go to https://console.cloud.google.com
2. Sign in with your Google account
3. In the top bar, click the project dropdown (it says "Select a project" or shows a project name)
4. Click **"New Project"** in the top right of the popup
5. Project name: `TimeReg`
6. Organization: leave as default
7. Click **"Create"**
8. Wait a few seconds, then click the project dropdown again and select **"TimeReg"**

## Step 2: Enable Google Sheets API

1. In the left sidebar, click **"APIs & Services"** → **"Library"**
   - Or go directly to: https://console.cloud.google.com/apis/library
2. In the search bar, type `Google Sheets API`
3. Click on **"Google Sheets API"** in the results
4. Click the blue **"Enable"** button
5. Wait for it to enable (you'll be redirected to the API overview page)

## Step 3: Enable Google Calendar API

1. Go back to the API Library: click **"APIs & Services"** → **"Library"** in the sidebar
2. In the search bar, type `Google Calendar API`
3. Click on **"Google Calendar API"** in the results
4. Click the blue **"Enable"** button

## Step 4: Configure OAuth Consent Screen

Before creating credentials, Google requires you to set up a consent screen.

1. In the left sidebar, click **"APIs & Services"** → **"OAuth consent screen"**
   - Or go to: https://console.cloud.google.com/apis/credentials/consent
2. Select **"External"** (the only option unless you have a Google Workspace)
3. Click **"Create"**
4. Fill in the form:
   - **App name**: `TimeReg`
   - **User support email**: select your email from the dropdown
   - **App logo**: skip (leave empty)
   - Scroll down to **"Developer contact information"**
   - **Email addresses**: enter your email
5. Click **"Save and Continue"**

### Scopes page

6. Click **"Add or Remove Scopes"**
7. In the filter box, type `spreadsheets`
8. Check the box for: `https://www.googleapis.com/auth/spreadsheets`
9. Clear the filter, type `calendar`
10. Check the box for: `https://www.googleapis.com/auth/calendar.events`
11. Click **"Update"** at the bottom
12. Click **"Save and Continue"**

### Test users page

13. Click **"+ Add Users"**
14. Enter your Gmail/Google email address
15. Click **"Add"**
16. Click **"Save and Continue"**

### Summary page

17. Review everything, then click **"Back to Dashboard"**

## Step 5: Create OAuth Credentials

1. In the left sidebar, click **"APIs & Services"** → **"Credentials"**
   - Or go to: https://console.cloud.google.com/apis/credentials
2. Click **"+ Create Credentials"** at the top
3. Select **"OAuth client ID"**
4. **Application type**: select **"Desktop app"**
5. **Name**: `TimeReg CLI`
6. Click **"Create"**

### Download the credentials

7. A popup appears with your Client ID and Client Secret
8. Click **"Download JSON"** (the download icon on the right)
9. A file like `client_secret_123456.json` is downloaded

## Step 6: Save the Credentials File

Move the downloaded file to the TimeReg config directory:

```bash
mkdir -p ~/.config/timereg
mv ~/Downloads/client_secret_*.json ~/.config/timereg/credentials.json
```

Verify it's there:

```bash
ls -la ~/.config/timereg/credentials.json
```

## Step 7: Authenticate

Run in your terminal:

```bash
timereg auth
```

This will:
1. Print a long URL
2. Copy that URL and open it in your browser
3. Google shows a warning: "This app isn't verified"
   - Click **"Advanced"** (small text at the bottom left)
   - Click **"Go to TimeReg (unsafe)"**
   - This is safe — it's your own app accessing your own account
4. Click **"Continue"** to grant Sheets and Calendar access
5. Google shows an authorization code
6. Copy the code and paste it back into your terminal
7. Done! Token is saved to `~/.config/timereg/token.json`

## Step 8: Set Up Spreadsheet and Calendar

### Option A: Let TimeReg create them for you

```bash
timereg config create-spreadsheet "TimeReg 2026"
timereg config create-calendar "Work Log"
```

### Option B: Use existing ones

For a spreadsheet you already have:
```bash
timereg config set spreadsheet_id "https://docs.google.com/spreadsheets/d/YOUR_ID/edit"
```

For your primary calendar:
```bash
timereg config set calendar_id "your.email@gmail.com"
```

## Step 9: Verify

```bash
timereg guide status
```

Everything should show `[v]`. You're ready to use TimeReg!

```bash
timereg start "First task"
timereg sync
```

## Troubleshooting

### "credentials.json not found"
Make sure the file is at `~/.config/timereg/credentials.json` and not still named `client_secret_*.json`.

### "This app is blocked" or "Access denied"
You forgot to add yourself as a test user in Step 4. Go back to the OAuth consent screen and add your email under "Test users".

### "Token has been expired or revoked"
Run `timereg auth` again to get a new token.

### Sync not working
Run `timereg sync` manually to see error messages. Check `timereg guide status` to verify all config is set.
