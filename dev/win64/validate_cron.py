#!/usr/bin/env python3
"""
Cron validation and testing utility for TARR Annunciator
"""
import json
import os
from apscheduler.triggers.cron import CronTrigger

def validate_cron_expression(cron_expr):
    """Validate a cron expression and return helpful feedback"""
    try:
        parts = cron_expr.split()
        if len(parts) != 5:
            return False, f"Cron expression must have exactly 5 parts (minute hour day month day_of_week), got {len(parts)}"
        
        minute, hour, day, month, day_of_week = parts
        
        # Try to create a CronTrigger to validate
        trigger = CronTrigger(
            minute=minute,
            hour=hour, 
            day=day if day != '*' else None,
            month=month if month != '*' else None,
            day_of_week=day_of_week if day_of_week != '*' else None
        )
        
        return True, "Valid cron expression"
        
    except Exception as e:
        return False, f"Invalid cron expression: {e}"

def check_cron_file():
    """Check all cron expressions in cron.json"""
    cron_file = 'json/cron.json'
    
    if not os.path.exists(cron_file):
        print("❌ cron.json file not found")
        return
    
    with open(cron_file, 'r') as f:
        cron_data = json.load(f)
    
    print("🕒 Checking Cron Expressions")
    print("=" * 40)
    
    all_valid = True
    
    # Check station announcements
    for i, item in enumerate(cron_data.get('station_announcements', [])):
        cron_expr = item.get('cron', '')
        enabled = item.get('enabled', False)
        valid, message = validate_cron_expression(cron_expr)
        
        status = "✅" if valid else "❌"
        enabled_text = "(ENABLED)" if enabled else "(disabled)"
        
        print(f"{status} Station {i+1}: '{cron_expr}' {enabled_text}")
        if not valid:
            print(f"   → {message}")
            all_valid = False
    
    # Check promo announcements  
    for i, item in enumerate(cron_data.get('promo_announcements', [])):
        cron_expr = item.get('cron', '')
        enabled = item.get('enabled', False)
        valid, message = validate_cron_expression(cron_expr)
        
        status = "✅" if valid else "❌"
        enabled_text = "(ENABLED)" if enabled else "(disabled)"
        
        print(f"{status} Promo {i+1}: '{cron_expr}' {enabled_text}")
        if not valid:
            print(f"   → {message}")
            all_valid = False
    
    # Check safety announcements
    for i, item in enumerate(cron_data.get('safety_announcements', [])):
        cron_expr = item.get('cron', '')
        enabled = item.get('enabled', False)
        valid, message = validate_cron_expression(cron_expr)
        
        status = "✅" if valid else "❌"
        enabled_text = "(ENABLED)" if enabled else "(disabled)"
        
        print(f"{status} Safety {i+1}: '{cron_expr}' {enabled_text}")
        if not valid:
            print(f"   → {message}")
            all_valid = False
    
    print("\n" + "=" * 40)
    if all_valid:
        print("✅ All cron expressions are valid!")
    else:
        print("❌ Some cron expressions need fixing")
        print("\nCommon cron patterns:")
        print("• Every 2 minutes: */2 * * * *")
        print("• Every hour: 0 * * * *")
        print("• Daily at 8 AM: 0 8 * * *")
        print("• Weekdays at noon: 0 12 * * 1-5")
        print("• Every 15 minutes: */15 * * * *")

if __name__ == '__main__':
    check_cron_file()
