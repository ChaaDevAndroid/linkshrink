@echo off
REM Keep variables local to this script so they do not leak into your terminal.
setlocal

REM Project root: this script lives in scripts, so go one folder up.
for %%I in ("%~dp0..") do set "ROOT=%%~fI"

REM First argument is the environment, second is the action.
set "TARGET=%~1"
set "ACTION=%~2"
if "%TARGET%"=="" set "TARGET=dev"
if "%ACTION%"=="" set "ACTION=up"

REM Help must work before any file checks.
if /I "%TARGET%"=="help" goto :usage
if /I "%ACTION%"=="help" goto :usage

REM Each target picks an env file. prod-debug reuses the prod one.
set "ENVFILE="
if /I "%TARGET%"=="dev"        set "ENVFILE=.env.dev"
if /I "%TARGET%"=="prod"       set "ENVFILE=.env.prod"
if /I "%TARGET%"=="prod-debug" set "ENVFILE=.env.prod"
if "%ENVFILE%"=="" goto :usage

REM Stop early with a clear message if a required file is missing.
if not exist "%ROOT%\%ENVFILE%" (
  echo Missing file: %ENVFILE%
  exit /b 1
)
if /I "%TARGET%"=="prod-debug" if not exist "%ROOT%\compose.prod-debug.yaml" (
  echo Missing file: compose.prod-debug.yaml
  exit /b 1
)

REM Flags shared by every docker compose command. Later files override earlier ones.
set BASE=--project-directory "%ROOT%" --env-file "%ROOT%\%ENVFILE%" -f "%ROOT%\compose.yaml"
if /I "%TARGET%"=="dev"        set BASE=%BASE% -f "%ROOT%\compose.dev.yaml"
if /I "%TARGET%"=="prod"       set BASE=%BASE% -f "%ROOT%\compose.prod.yaml"
if /I "%TARGET%"=="prod-debug" set BASE=%BASE% -f "%ROOT%\compose.prod.yaml" -f "%ROOT%\compose.prod-debug.yaml"

REM Jump to the label that matches the action.
if /I "%ACTION%"=="up"      goto :up
if /I "%ACTION%"=="down"    goto :down
if /I "%ACTION%"=="restart" goto :restart
if /I "%ACTION%"=="ps"      goto :ps
if /I "%ACTION%"=="logs"    goto :logs
if /I "%ACTION%"=="config"  goto :config
if /I "%ACTION%"=="reset"   goto :reset
if /I "%ACTION%"=="close"   goto :close
goto :usage

:up
if /I "%TARGET%"=="dev"  goto :up_dev
if /I "%TARGET%"=="prod" goto :up_prod
goto :up_debug

:up_dev
REM Dev: foreground, rebuild the api when code files change.
docker compose %BASE% up --build --watch
exit /b %errorlevel%

:up_prod
REM Prod: detached, so the terminal is free.
docker compose %BASE% up -d --build
exit /b %errorlevel%

:up_debug
REM Prod plus the override that publishes Postgres on 127.0.0.1 port 5433 only.
docker compose %BASE% up -d --build
if errorlevel 1 exit /b 1
echo.
echo prod-debug is up. Postgres is published on 127.0.0.1:5433, on this machine only.
echo Close the port when you are done: prod-debug close
exit /b 0

:close
REM Recreate prod without the debug override, which removes the published port.
if /I not "%TARGET%"=="prod-debug" goto :close_wrong
docker compose --project-directory "%ROOT%" --env-file "%ROOT%\.env.prod" -f "%ROOT%\compose.yaml" -f "%ROOT%\compose.prod.yaml" up -d
if errorlevel 1 exit /b 1
echo Port closed. Prod runs without a published database port.
exit /b 0

:close_wrong
echo close only applies to prod-debug.
exit /b 1

:down
REM Remove the containers and network. The data volume is kept.
docker compose %BASE% down
exit /b %errorlevel%

:restart
REM Restart only the api container, not the database.
docker compose %BASE% restart api
exit /b %errorlevel%

:ps
docker compose %BASE% ps
exit /b %errorlevel%

:logs
REM Follow the api logs. Ctrl+C stops following, not the app.
docker compose %BASE% logs -f api
exit /b %errorlevel%

:config
REM Print the merged configuration without starting anything.
docker compose %BASE% config
exit /b %errorlevel%

:reset
REM Deleting the volume is only allowed on dev.
if /I "%TARGET%"=="dev" goto :reset_dev
echo reset is blocked on %TARGET%, it would delete the database volume.
exit /b 1

:reset_dev
docker compose %BASE% down -v
exit /b %errorlevel%

:usage
echo Usage: dev, prod or prod-debug, then an action
echo.
echo Actions:
echo   up        build and start. dev runs in the foreground with watch, prod in the background
echo   down      stop and remove containers, keep the data
echo   restart   restart the api container only
echo   ps        show container status
echo   logs      follow the api logs
echo   config    print the merged compose configuration
echo   reset     dev only: delete containers AND the database volume
echo   close     prod-debug only: remove the published database port
echo   help      show this text
exit /b 1