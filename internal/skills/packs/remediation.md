# Remediation Cheatsheet

## Secrets

### Hardcoded API keys / tokens
1. Immediately rotate the exposed credential in the provider dashboard.
2. Remove the hardcoded value from source code.
3. Use environment variables or a secrets manager (AWS Secrets Manager, HashiCorp Vault, GCP Secret Manager, Doppler).
4. Add a `.gitignore` rule for `.env` files and a pre-commit hook (e.g. `detect-secrets` or `gitleaks`).

### Private keys in source
1. Revoke the key immediately.
2. Generate a new key pair; store the private key only in a secrets manager or CI/CD secret store.
3. Audit git history with `git log --all -S 'BEGIN PRIVATE KEY'` and consider a force-push + history rewrite if necessary.

## Vulnerable Dependencies

### npm / Node.js
```
npm audit fix
# For breaking changes: npm audit fix --force (review carefully)
```

### Python / pip
```
pip install --upgrade <package>
# or pin to a safe version in requirements.txt
```

### Go modules
```
go get <module>@<safe-version>
go mod tidy
```

## Code Vulnerabilities

### SQL Injection
- Use parameterised queries / prepared statements — never format user input into SQL strings.
- ORM frameworks: use the ORM's query builder, not raw string interpolation.

### Command Injection
- Avoid `exec.Command` / `subprocess` with user-controlled input.
- Prefer allowlist validation; use argument arrays not shell strings.

### Path Traversal
- Sanitise paths with `filepath.Clean` then verify the result is under the expected root.
- Use `filepath.Rel` to check the cleaned path does not escape the root.

### SSRF (Server-Side Request Forgery)
- Maintain an allowlist of permitted outbound hosts.
- Block requests to RFC-1918 / link-local ranges (10.x, 172.16–31.x, 192.168.x, 169.254.x).

### Insecure Deserialization
- Avoid deserializing untrusted data into complex object graphs.
- For JSON, use strict schema validation; for binary formats, prefer length-delimited formats with schema versioning.

## Misconfigurations

### Debug mode in production
- Ensure `DEBUG=false` / `ENV=production` in deployment environment.
- Gate debug endpoints behind authentication or remove them entirely.

### Overly permissive CORS
- Set `Access-Control-Allow-Origin` to specific allowed origins, not `*`.
- Never combine `Access-Control-Allow-Credentials: true` with `Allow-Origin: *`.

### Default credentials
- Change all default passwords at first run.
- Enforce strong password policy and MFA where applicable.
