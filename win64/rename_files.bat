@echo off
REM Rename working files to cleaner names after cleanup

echo ===============================================
echo Renaming files to cleaner names...
echo ===============================================

REM Rename the working templates to standard names
if exist "templates\index_fixed.html" (
    if exist "templates\index.html" del "templates\index.html"
    ren "templates\index_fixed.html" "index.html"
    echo ✓ Renamed index_fixed.html to index.html
)

if exist "templates\admin_fixed2.html" (
    if exist "templates\admin.html" del "templates\admin.html"
    ren "templates\admin_fixed2.html" "admin.html"
    echo ✓ Renamed admin_fixed2.html to admin.html
)

REM Rename the main app file
if exist "app_pygame.py" (
    if exist "app.py" del "app.py"
    ren "app_pygame.py" "app.py"
    echo ✓ Renamed app_pygame.py to app.py
)

echo.
echo ===============================================
echo File renaming complete!
echo ===============================================
echo.
echo Your application now uses standard file names:
echo - app.py (main application)
echo - templates\index.html (main interface)
echo - templates\admin.html (admin interface)
echo.

pause
