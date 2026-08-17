README - Deploy & Local Run

This document collects the exact commands to run locally, build, and deploy the API and frontend.

Local dev - API (Go)
1) Install Go 1.21
2) From repository root, preferred quick start (Windows PowerShell):
   .\scripts\run-local.ps1

Or (cross-platform manual):
   # API
   cd api
   go mod download
   # Ensure ADMIN_PASSKEY is set before running the API (no default). Example in PowerShell:
   #   $env:ADMIN_PASSKEY = "your_local_passkey_here"
   # Or in bash:
   #   export ADMIN_PASSKEY=your_local_passkey_here
   $env:PORT=8080
   go run main.go

   # Frontend (run in separate terminal)
   cd admin-portal
   npm ci
   npm run dev

Health check:
   curl http://localhost:8080/status

Local dev - Frontend (Vite)
1) Node.js (16+)
   cd admin-portal
   npm ci
   npm run dev
2) Open browser to Vite dev URL (printed in terminal)
   The Vite dev server proxies /api to http://localhost:8080

Scripts
- Windows: .\scripts\run-local.ps1  (opens two PowerShell windows and starts API+frontend — requires ADMIN_PASSKEY to be set in the environment)
- Unix/macOS: ./scripts/run-local.sh (starts API+frontend in background — requires ADMIN_PASSKEY to be set in the environment)

Build frontend for GitHub Pages (static)
cd admin-portal
# ensure VITE_API_BASE is set (in CI this comes from GitHub Secrets)
npm run build
# output will be in admin-portal/dist

Build and run Docker image locally (API)
# from repo root
cd api
GOOS=linux GOARCH=amd64 go build -o api_server main.go
# build a Docker image
docker build -t ghcr.io/<owner>/ucs503-api:local -f Dockerfile .
# run locally
docker run -e ADMIN_PASSKEY="your-secure-passkey" -e PORT=8080 -p 8080:8080 ghcr.io/<owner>/ucs503-api:local

Push image to GHCR (manual)
# Tag and push
docker tag ghcr.io/<owner>/ucs503-api:local ghcr.io/<owner>/ucs503-api:latest
# Login
echo $GHCR_PAT | docker login ghcr.io -u <your-github-username> --password-stdin
docker push ghcr.io/<owner>/ucs503-api:latest

Deploy on EC2 with docker-compose
# On EC2 create folder ~/ucs503, copy docker-compose.ec2.yml and .env
# Populate .env with production values (see .env.example)
cd ~/ucs503
docker compose pull
docker compose up -d

Verify
- curl https://api.yourdomain.com/status
- use the Admin Portal to log in and upload archives
- check S3 console for objects
- check RDS for metadata

Rollback
- docker pull ghcr.io/<owner>/ucs503-api:<previous-tag>
- update docker-compose to use that tag and docker compose up -d

Notes
- Use the provided .env.example as a template on the server (do NOT commit secrets)
- For CI and deploy, use the GitHub workflows already added to this repository
