param(
    [string]$Output = "Musicle.exe"
)

# Embed icon from assets/MusicLe.png into .syso resource file
go-winres simply --icon assets/MusicLe.png
if (-not $?) { exit 1 }

# Build the application
go build -o $Output .
if (-not $?) { exit 1 }

Write-Host "Built: $Output"
