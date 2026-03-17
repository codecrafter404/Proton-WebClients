# Mobile Apps

This directory contains the mobile applications for Proton Calendar.

## Android Calendar

The Android Calendar app is included as a **git submodule** pointing to the official [ProtonMail/android-calendar](https://github.com/ProtonMail/android-calendar) repository.

### Prerequisites

- **Android Studio** (latest stable version)
- **JDK 17**
- **Android SDK** (API level 35)

### Setup

1. **Clone with submodules**:
   ```bash
   git clone --recurse-submodules https://github.com/codecrafter404/Proton-WebClients.git
   ```

   Or if already cloned:
   ```bash
   git submodule update --init --recursive
   ```

2. **Configure the backend server address**:

   Create a `local.properties` file in `mobile/android-calendar/`:
   ```properties
   # For Android emulator connecting to host machine backend:
   HOST=10.0.2.2:8080

   # For a physical device on the same network:
   # HOST=192.168.1.100:8080

   # For a remote server:
   # HOST=calendar.example.com

   sdk.dir=/path/to/Android/Sdk
   ```

3. **Open in Android Studio**:
   Open `mobile/android-calendar/` as a project in Android Studio.

4. **Build and run**:
   Select the `devDebug` build variant and run on your device/emulator.

### Build Variants

| Variant       | Description |
|:------------- |:----------- |
| `devDebug`    | Debug build pointing to custom backend (configured via `HOST` in `local.properties`) |
| `devRelease`  | Release build pointing to custom backend |
| `prodDebug`   | Debug build pointing to Proton production servers |
| `prodRelease` | Release build pointing to Proton production servers |

### Setting the Dev Server Address

The `HOST` property in `local.properties` controls which backend the `dev` flavors connect to.

**Android Emulator** → Use `10.0.2.2:8080` (maps to host machine's `localhost:8080`)

**Physical Device** → Use your computer's local IP, e.g., `192.168.1.100:8080`

**Remote Server** → Use your server's domain/IP, e.g., `calendar.example.com`

### CI/CD

The GitHub Actions workflow (`.github/workflows/mobile.yml`) builds the APK automatically:

- **Manual trigger**: Go to Actions → Mobile Android Build → Run workflow
  - Set `backend_url` to your server address
  - Select `build_variant` (devDebug, devRelease, etc.)
- **Automatic trigger**: On push/PR changes to `mobile/` or the workflow file

The built APK is uploaded as a workflow artifact for download.

### Syncing with Upstream

To update the Android Calendar app to the latest upstream version:

```bash
cd mobile/android-calendar
git fetch origin
git checkout origin/release/2.29.0-332  # or the latest release branch
cd ../..
git add mobile/android-calendar
git commit -m "Update android-calendar submodule to latest release"
```

### API Compatibility

The Android Calendar app uses the Proton Calendar REST API with the `calendar-api` prefix (configured via `protonEnvironment { apiPrefix = "calendar-api" }`). The Go backend in this repository implements the same API endpoints:

| Android App Endpoint | Backend Route |
|:--------------------|:-------------|
| `POST /core/v4/auth` | Login/authentication |
| `GET /calendar/v1` | List calendars |
| `POST /calendar/v1` | Create calendar |
| `PUT /calendar/v1/{id}/events/sync` | Create/update/delete events |
| `GET /calendar/v1/{id}/events` | List events |
| `GET /calendar/v1/{id}/members` | List members |
| `GET /calendar/v1/{id}/settings` | Calendar settings |
| `GET /settings/calendar` | User settings |
