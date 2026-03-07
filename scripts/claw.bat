@echo off
REM ============================================================
REM StarClaw CLI for Windows — shortcut for daily operations
REM Usage: claw <command>
REM
REM Install: Add the scripts\ folder to your PATH, or copy
REM          this file to a directory already in PATH.
REM ============================================================

setlocal

REM Auto-detect project root
set "SCRIPT_DIR=%~dp0"
pushd "%SCRIPT_DIR%\.."
set "PROJECT_ROOT=%CD%"

if "%1"=="" goto help
set CMD=%1
shift

if "%CMD%"=="up"          goto make_cmd
if "%CMD%"=="up-cn"       goto make_cmd
if "%CMD%"=="start"       goto make_cmd
if "%CMD%"=="stop"        goto make_cmd
if "%CMD%"=="restart"     goto make_cmd
if "%CMD%"=="down"        goto make_cmd
if "%CMD%"=="logs"        goto make_cmd
if "%CMD%"=="logs-api"    goto make_cmd
if "%CMD%"=="logs-web"    goto make_cmd
if "%CMD%"=="logs-mysql"  goto make_cmd
if "%CMD%"=="logs-redis"  goto make_cmd
if "%CMD%"=="ps"          goto make_cmd
if "%CMD%"=="status"      goto ps_cmd
if "%CMD%"=="stats"       goto make_cmd
if "%CMD%"=="health"      goto make_cmd
if "%CMD%"=="ping"        goto health_cmd
if "%CMD%"=="update"      goto make_cmd
if "%CMD%"=="pull"        goto update_cmd
if "%CMD%"=="update-cn"   goto make_cmd
if "%CMD%"=="rebuild-api" goto make_cmd
if "%CMD%"=="rebuild-web" goto make_cmd
if "%CMD%"=="backup"      goto make_cmd
if "%CMD%"=="shell"       goto shell_cmd
if "%CMD%"=="sh"          goto shell_cmd
if "%CMD%"=="mysql"       goto mysql_cmd
if "%CMD%"=="redis"       goto redis_cmd
if "%CMD%"=="prune"       goto make_cmd
if "%CMD%"=="clean"       goto prune_cmd
if "%CMD%"=="init"        goto make_cmd
if "%CMD%"=="la"          goto la_cmd
if "%CMD%"=="lw"          goto lw_cmd
if "%CMD%"=="ra"          goto ra_cmd
if "%CMD%"=="rw"          goto rw_cmd
if "%CMD%"=="version"     goto version_cmd
if "%CMD%"=="v"           goto version_cmd
if "%CMD%"=="-v"          goto version_cmd
if "%CMD%"=="help"        goto help
if "%CMD%"=="-h"          goto help
if "%CMD%"=="--help"      goto help
goto fallback

:ps_cmd
set CMD=ps
goto make_cmd

:health_cmd
set CMD=health
goto make_cmd

:update_cmd
set CMD=update
goto make_cmd

:prune_cmd
set CMD=prune
goto make_cmd

:shell_cmd
make shell-api
goto end

:mysql_cmd
make shell-mysql
goto end

:redis_cmd
make shell-redis
goto end

:la_cmd
make logs-api
goto end

:lw_cmd
make logs-web
goto end

:ra_cmd
make rebuild-api
goto end

:rw_cmd
make rebuild-web
goto end

:version_cmd
echo StarClaw CLI
curl -sf http://localhost:8080/v1/version 2>nul || echo API not running
goto end

:make_cmd
make %CMD%
goto end

:fallback
echo Passing to make: %CMD%
make %CMD%
goto end

:help
echo.
echo  StarClaw CLI -- Daily operations made easy
echo.
echo  Usage: claw ^<command^>
echo         starclaw ^<command^>
echo.
echo  Lifecycle:
echo    up            Build and start all services
echo    up-cn         Build and start (China mirror)
echo    start         Start existing containers
echo    stop          Stop all containers
echo    restart       Restart all containers
echo    down          Stop and remove containers
echo    destroy       Stop, remove, AND delete data
echo.
echo  Logs:
echo    logs          Follow all logs
echo    logs-api, la  Follow API logs
echo    logs-web, lw  Follow Web logs
echo    logs-mysql    Follow MySQL logs
echo    logs-redis    Follow Redis logs
echo.
echo  Status:
echo    ps, status    Show running containers
echo    stats         Show CPU/MEM usage
echo    health, ping  Check API health
echo    version, v    Show version info
echo.
echo  Update:
echo    update, pull  Pull latest + rebuild
echo    update-cn     Pull + rebuild (China)
echo    rebuild-api   Rebuild API only
echo    rebuild-web   Rebuild Web only
echo.
echo  Data:
echo    backup        Backup database + data
echo    restore-db    Restore from backup
echo.
echo  Shell:
echo    shell, sh     Open API container shell
echo    mysql         Open MySQL CLI
echo    redis         Open Redis CLI
echo.
echo  Other:
echo    init          First-time setup
echo    prune, clean  Remove unused Docker images
echo.

:end
popd
endlocal
