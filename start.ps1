[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$env:PYTHONIOENCODING = "utf-8"

Set-Location $PSScriptRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Host "go not found. Please install Go first." -ForegroundColor Red
  exit 1
}

if (-not (Test-Path (Join-Path $PSScriptRoot ".env"))) {
  Write-Host ".env not found. You can copy .env.example to .env for local configuration." -ForegroundColor Yellow
}

Write-Host "Starting LeetCodeClaw API on http://127.0.0.1:10170 ..." -ForegroundColor Cyan
go run ./cmd/leetcode-api
exit $LASTEXITCODE
