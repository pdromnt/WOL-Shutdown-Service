:: Self-elevate to Administrator
@echo off
:: Check for admin rights
net session >nul 2>&1
if %errorlevel%==0 (
    goto gotAdmin
) else (
    echo Requesting administrative privileges. Will open in a new window!
    powershell -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b
)

:gotAdmin

@echo off
:menu
cls
echo ==============================
echo   WOL Shutdown Service Menu
echo ==============================
echo 1. Install Service
echo 2. Uninstall Service
echo 3. Exit
echo ==============================
set /p choice=Choose an option: 

if "%choice%"=="1" goto install
if "%choice%"=="2" goto uninstall
if "%choice%"=="3" goto end
goto menu

:install
echo Installing service...
REM %~dp0 expands to the folder where this .bat file lives
sc create "WOLShutdownService" binPath=%~dp0WolShutdownService.exe start=auto
sc description "WOLShutdownService" "Listens for WOL packets and shuts down the PC"
sc failure "WOLShutdownService" reset=60 actions=restart/5000

REM Register Event Log source
reg add "HKLM\SYSTEM\CurrentControlSet\Services\EventLog\Application\WOLShutdownService" /v EventMessageFile /t REG_EXPAND_SZ /d "%SystemRoot%\System32\EventCreate.exe" /f

sc start "WOLShutdownService"
echo Service installed, started, and Event Log source registered.
pause
goto menu

:uninstall
echo Stopping and removing service...
sc stop "WOLShutdownService"
sc delete "WOLShutdownService"
reg delete "HKLM\SYSTEM\CurrentControlSet\Services\EventLog\Application\WOLShutdownService" /f
echo Service and Event Log source removed.
pause
goto menu

:end
echo Exiting...
