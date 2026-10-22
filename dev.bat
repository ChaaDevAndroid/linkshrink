REM Do not print each command.
@echo off
REM Run the engine with "dev" as the first argument. %* forwards everything you typed
REM after "dev" (the action). %~dp0 is this file's folder, so it works from anywhere.
call "%~dp0scripts\run.bat" dev %*