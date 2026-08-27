Final Deployment Checklist — Thermodynamic-Evolution-UCS503

This checklist gathers the final items to confirm before pushing to GitHub and deploying to AWS (EC2 + RDS + S3). Follow each step and only push when everything in the "Pre-push verification" section is complete.

1) Pre-push verification (local)
- [ ] Run Go static checks and build
  - cd api
  - go mod download
  - go vet ./...
  - go build ./...
- [ ] Run frontend checks and build
  - cd admin-portal
  - npm ci
  - npm run build
- [ ] Run linter/tests if you have them (add as needed)

2) Repository hygiene
- [ ] Remove any sensitive data from the working tree (no .env with secrets, no AWS keys, no PEM files)
- [ ] Ensure archives.json does not contain secrets. If it contains paths/URLs that are private, consider removing before push or migrating to DB.
- [ ] Update top-level README with a short note about deployment (optional)

3) GitHub secrets to add (see GITHUB_SECRETS_COPY.txt)
- ADMIN_PASSKEY
- AWS_ACCESS_KEY_ID
- AWS_SECRET_ACCESS_KEY
- S3_BUCKET
- S3_REGION
- DATABASE_URL
- GHCR_PAT
- VITE_API_BASE
- EC2_HOST
- EC2_USER
- EC2_SSH_PRIVATE_KEY (SSH private key contents)
- (Optional) S3_PRESIGN_EXPIRE_HOURS

4) CI / Workflows
- [ ] Confirm .github/workflows/ci-go.yml exists and triggers on api changes
- [ ] Confirm .github/workflows/publish-api-image.yml builds & pushes image to GHCR
- [ ] Confirm .github/workflows/deploy-to-ec2.yml has EC2 secrets configured
- [ ] Confirm .github/workflows/deploy-frontend.yml contains VITE_API_BASE secret usage

5) Server (EC2) preparation checklist
- [ ] EC2 instance up (Ubuntu recommended) with Docker & docker compose installed
- [ ] .env file created on server (see .env.example) and owned by appropriate user with permissions 600
- [ ] docker-compose.ec2.yml (or your compose file) placed on server and uses image ghcr.io/<owner>/ucs503-api:latest
- [ ] Nginx or ALB configured to reverse-proxy to API (optional but recommended)
- [ ] SSL cert obtained with certbot or via ACM/load balancer

6) Post-deploy checks (after push & deploy)
- [ ] Curl the health endpoint:
  curl https://api.yourdomain.com/status
- [ ] Upload test file via Admin Portal and confirm S3 object appears (or local uploads dir)
- [ ] Confirm metadata record in RDS (psql) or archives.json updated
- [ ] Confirm download link works (presigned URL or direct object URL)

7) Rollback plan
- [ ] Keep the previous Docker image tag available on GHCR to roll back quickly
- [ ] Backup archives.json (if still using local storage) and a DB snapshot for RDS before migration

Notes
- Do not commit .env or any private keys. Use GitHub Secrets for CI and server-side management tools (AWS Secrets Manager, Parameter Store) for production.
- If using EC2 instance roles, you can avoid storing AWS keys on the server.

If you want, I can run through this checklist with you interactively and help set each secret and run the first push. Tell me which step to begin with.