REM Do not print each command.
@echo off
REM Same as dev.bat, but with "prod" as the first argument.
call "%~dp0scripts\run.bat" prod %*