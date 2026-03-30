@echo off
echo ============================================
echo  Extractor Server D Auto Setup
echo ============================================

echo [1/6] Creating directories...
mkdir C:\extractor 2>nul
mkdir C:\extractor\data 2>nul
mkdir C:\extractor\logs 2>nul

echo [2/6] Extracting code...
cd /d C:\extractor
powershell -Command "Expand-Archive -Path 'C:\Users\Administrator\Desktop\extractor-deploy.zip' -DestinationPath 'C:\extractor' -Force"
echo Done. Listing:
dir C:\extractor

echo [3/6] Checking Python...
python --version 2>nul
if errorlevel 1 (
    echo Python NOT found! Please install Python 3.11+
    echo Download: https://www.python.org/downloads/
    echo IMPORTANT: Check "Add Python to PATH" during install
    pause
    exit /b 1
)

echo [4/6] Installing Python dependencies...
cd /d C:\extractor\bridge
pip install fastapi uvicorn pydantic pyyaml httpx numpy pandas

echo [5/6] Checking xtquant...
python -c "from xtquant import xtdata; print('xtquant OK')" 2>nul
if errorlevel 1 (
    echo xtquant not found - will run in MOCK mode
    echo To fix: copy xtquant folder from QMT install to Python site-packages
)

echo [6/6] Setting up SSH authorized_keys...
mkdir C:\Users\Administrator\.ssh 2>nul
if not exist C:\ProgramData\ssh\administrators_authorized_keys (
    echo Creating admin authorized_keys file...
    copy nul C:\ProgramData\ssh\administrators_authorized_keys 2>nul
)

echo ============================================
echo  Setup complete!
echo  Next: run start-bridge.bat and start-api.bat
echo ============================================

echo Creating start scripts...

echo @echo off > C:\extractor\start-bridge.bat
echo cd /d C:\extractor\bridge >> C:\extractor\start-bridge.bat
echo python main.py >> C:\extractor\start-bridge.bat

echo @echo off > C:\extractor\test-bridge.bat
echo curl http://localhost:8098/health >> C:\extractor\test-bridge.bat

pause
