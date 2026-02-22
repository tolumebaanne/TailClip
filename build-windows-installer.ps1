$ErrorActionPreference = "Stop"

# Get current directory
$ProjectDir = (Get-Item .).FullName
$AgentDir = Join-Path $ProjectDir "agent"
$DistDir = Join-Path $ProjectDir "distribution"
$InstallerDir = Join-Path $ProjectDir "installer"

Write-Host "--- TailClip Windows Installer Build ---"

# Step 1: Create distribution folder if it doesn't exist
if (-not (Test-Path $DistDir)) {
    Write-Host "Creating distribution directory..."
    New-Item -ItemType Directory -Path $DistDir | Out-Null
}

# Step 2: Build the Go agent
Write-Host "Building Go agent for Windows natively..."
Set-Location $AgentDir
go env -w GOOS=windows
go env -w GOARCH=amd64
# compile with -ldflags "-H=windowsgui" so it doesn't open console, wait no we want it a service or via VBS wrapper
# actually using VBS wrapper is fine, but building with windowsgui is even better:
go build -ldflags "-H=windowsgui -w -s" -o "$DistDir\agent.exe" .
if ($LASTEXITCODE -ne 0) {
    Write-Error "Go build failed!"
    exit 1
}
Set-Location $ProjectDir
Write-Host "Go agent built successfully."

# Step 3: Check for Inno Setup compiler
$ISCCPaths = @(
    "C:\Program Files (x86)\Inno Setup 6\ISCC.exe",
    "C:\Program Files\Inno Setup 6\ISCC.exe",
    "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
)

$ISCCPath = $null
foreach ($path in $ISCCPaths) {
    if (Test-Path $path) {
        $ISCCPath = $path
        break
    }
}

if ($null -eq $ISCCPath) {
    Write-Host "Inno Setup compiler not found. Please ensure it is installed."
    Write-Error "Inno Setup is required to build the installer."
    exit 1
}

# Step 4: Run Inno Setup Compiler
Write-Host "Compiling installer with Inno Setup..."
& $ISCCPath "$InstallerDir\tailclip.iss"
if ($LASTEXITCODE -ne 0) {
    Write-Error "Inno Setup compilation failed!"
    exit 1
}

Write-Host "--- Build Complete ---"
Write-Host "Installer is available in: $DistDir\TailClip_Installer.exe"
exit 0
