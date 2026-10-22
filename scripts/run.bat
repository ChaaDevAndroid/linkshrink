REM Do not print each command as it runs, only its output.
@echo off

REM Changes to variables stay inside this script and don't leak into your terminal.
setlocal

REM %~dp0 is the folder of this script (scripts\). Adding .. goes one level up, and
REM %%~fI turns it into a clean absolute path. ROOT is the project root, so the script
REM works no matter which folder you run it from.
for %%I in ("%~dp0..") do set "ROOT=%%~fI"

REM First argument is the environment, second is the action. %~1 strips surrounding quotes.
set "TARGET=%~1"
set "ACTION=%~2"

REM Defaults: plain "run.bat" means "dev up".
if "%TARGET%"=="" set "TARGET=dev"
if "%ACTION%"=="" set "ACTION=up"

REM "help" must work even before we check that env files exist.
if /I "%ACTION%"=="help" goto :usage

REM Reject any environment other than dev or prod (/I ignores upper/lower case).
if /I not "%TARGET%"=="dev" if /I not "%TARGET%"=="prod" goto :usage

REM Stop early with a clear message if the env file for this environment is missing.
if not exist "%ROOT%\.env.%TARGET%" (
  echo Missing file: .env.%TARGET%
  exit /b 1
)

REM Flags shared by every docker compose command:
REM   --project-directory  where relative paths in the compose files resolve from
REM   --env-file           supplies the ${POSTGRES_*} values
REM   -f ... -f ...        base file plus the environment override, merged in order
set BASE=--project-directory "%ROOT%" --env-file "%ROOT%\.env.%TARGET%" -f "%ROOT%\compose.yaml" -f "%ROOT%\compose.%TARGET%.yaml"

REM Jump to the label that matches the action.
if /I "%ACTION%"=="up"      goto :up
if /I "%ACTION%"=="down"    goto :down
if /I "%ACTION%"=="restart" goto :restart
if /I "%ACTION%"=="ps"      goto :ps
if /I "%ACTION%"=="logs"    goto :logs
if /I "%ACTION%"=="config"  goto :config
if /I "%ACTION%"=="reset"   goto :reset

REM Unknown action: show the help text.
goto :usage

:up
REM prod runs in the background, dev runs in the foreground.
if /I "%TARGET%"=="prod" goto :up_prod
REM Dev: build the image, start the containers, rebuild the api when code files change.
docker compose %BASE% up --build --watch
REM Pass docker's exit code back to the caller.
exit /b %errorlevel%

:up_prod
REM Prod: build the image and start the containers detached (-d) so the terminal is free.
docker compose %BASE% up -d --build
exit /b %errorlevel%

:down
REM Remove the containers and network. The data volume is kept.
docker compose %BASE% down
exit /b %errorlevel%

:restart
REM Restart only the api container, not the database.
docker compose %BASE% restart api
exit /b %errorlevel%

:ps
REM Show the status of this environment's containers.
docker compose %BASE% ps
exit /b %errorlevel%

:logs
REM Follow the api logs (-f). Ctrl+C stops following, not the app.
docker compose %BASE% logs -f api
exit /b %errorlevel%

:config
REM Print the final merged configuration without starting anything.
docker compose %BASE% config
exit /b %errorlevel%

:reset
REM Safety: never allow volume deletion on prod.
if /I "%TARGET%"=="prod" (
  echo reset is blocked on prod, it would delete the database volume.
  exit /b 1
)
REM Dev: remove containers, network AND the data volume, so migrations re-run next start.
docker compose %BASE% down -v
exit /b %errorlevel%

:usage
REM In echo, ^| prints a literal pipe and "echo." prints an empty line.
echo Usage: dev ^| prod ^<action^>
echo.
echo Actions:
echo   up        build and start (dev: foreground with watch, prod: background)
echo   down      stop and remove containers, keep the data
echo   restart   restart the api container only
echo   ps        show container status
echo   logs      follow the api logs
echo   config    print the merged compose configuration
echo   reset     dev only: delete containers AND the database volume
echo   help      show this text
exit /b 1