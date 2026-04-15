# WebBou Build Script for Windows

Write-Host "=== WebBou Build Script ===" -ForegroundColor Cyan

# Create bin directory
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

# Generate certificates if not exist
if (-not (Test-Path "cert.pem")) {
    Write-Host "`nGenerating TLS certificates..." -ForegroundColor Yellow
    
    # Try OpenSSL first
    $opensslExists = Get-Command openssl -ErrorAction SilentlyContinue
    
    if ($opensslExists) {
        openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj "/CN=localhost"
    } else {
        Write-Host "OpenSSL not found. Skipping certificate generation." -ForegroundColor Yellow
        Write-Host "You can install OpenSSL or generate certificates manually later." -ForegroundColor Yellow
        Write-Host "For testing, the server will generate self-signed certs automatically." -ForegroundColor Cyan
    }
}

# Build Go server
Write-Host "`nBuilding Go server..." -ForegroundColor Yellow
Set-Location server
go mod download
go mod tidy
go build -o ..\bin\server.exe main_webbou.go
Set-Location ..

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Go server built successfully" -ForegroundColor Green
} else {
    Write-Host "✗ Go server build failed" -ForegroundColor Red
    exit 1
}

# Build Rust client
Write-Host "`nBuilding Rust client..." -ForegroundColor Yellow
Set-Location client
cargo update
cargo build --release
Copy-Item target\release\webbou-client.exe ..\bin\client.exe
Set-Location ..

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Rust client built successfully" -ForegroundColor Green
} else {
    Write-Host "✗ Rust client build failed" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== Build Complete ===" -ForegroundColor Cyan
Write-Host "Server: .\bin\server.exe" -ForegroundColor Green
Write-Host "Client: .\bin\client.exe" -ForegroundColor Green
