@echo off
setlocal
cd /d "%~dp0"
go run ./certgen %*
exit /b %ERRORLEVEL%
