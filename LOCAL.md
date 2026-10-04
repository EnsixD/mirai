# Mirai: local development on Windows

The panel and PostgreSQL run as native Windows processes. All local data and runtime files live in `.local/`; this folder is excluded from version control.

From PowerShell in the project folder:

```powershell
.\mirai start
.\mirai stop
.\mirai build
.\mirai restart
.\mirai reset-password
.\mirai url
```

Open http://127.0.0.1:5173/login. The login and initial password are in `.local/login.txt`. After resetting the password, use the new password printed by the command.

Frontend changes reload automatically. After changing Go code, run `mirai build`, then `mirai restart`.

PostgreSQL listens on `127.0.0.1:55432`, the backend on `127.0.0.1:2053`, and Vite on `127.0.0.1:5173`. The development backend uses HTTP and disables the VPN node. Panel data is persisted in `.local/postgres-data`; VPN connections require a separately configured node.

The original source before the rename is backed up in `.local/pre-mirai-source.zip`.
