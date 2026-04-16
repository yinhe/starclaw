@echo off
REM StarClaw Claw — Deploy via GitHub (Windows)
REM Usage: scripts\deploy.bat [commit message] [target]
REM   target: api (default), web, all
REM Flow: sync to OSS repo → push GitHub → server git pull → docker rebuild

setlocal
set MSG=%~1
if "%MSG%"=="" set MSG=update
set TARGET=%~2
if "%TARGET%"=="" set TARGET=api

echo === [1/3] Sync to OSS repo ===
robocopy "E:\starclaw\claw" "E:\claw-oss" /E /XD node_modules .git data .vite /XF *.tar.gz *.exe mcp-bridge-* server.exe > nul
cd /d E:\claw-oss
git add -A
git diff --cached --quiet || git commit -m "%MSG%"
git push origin main
echo Pushed to GitHub

echo.
echo === [2/3] Deploy on server (target: %TARGET%) ===
ssh root@starclaw.me "bash /opt/starclaw/deploy-update.sh %TARGET%"

echo.
echo === [3/3] Done ===
echo https://app.starclaw.me
endlocal
