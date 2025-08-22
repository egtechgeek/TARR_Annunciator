#!/usr/bin/env python3
"""
API Test Script for TARR Annunciator
Test all API endpoints to ensure they're working correctly
"""
import requests
import json
import time

# Configuration
BASE_URL = "http://localhost:8080"
API_KEY = "tarr-api-2025"

def test_api_status():
    """Test the status endpoint (no auth required)"""
    print("Testing API Status...")
    try:
        response = requests.get(f"{BASE_URL}/api/status")
        if response.ok:
            data = response.json()
            print(f"✓ Status: {data['status']}")
            print(f"✓ Audio Available: {data['audio_available']}")
            print(f"✓ Volume: {data['volume']}%")
            return True
        else:
            print(f"✗ Status check failed: {response.status_code}")
            return False
    except Exception as e:
        print(f"✗ Status check error: {e}")
        return False

def test_station_announcement():
    """Test station announcement API"""
    print("\nTesting Station Announcement...")
    try:
        response = requests.post(
            f"{BASE_URL}/api/announce/station",
            headers={'X-API-Key': API_KEY},
            json={
                'train_number': '1',
                'direction': 'westbound',
                'destination': 'goodwin_station',
                'track_number': '1'
            }
        )
        if response.ok:
            data = response.json()
            print(f"✓ Station announcement triggered: {data['message']}")
            return True
        else:
            print(f"✗ Station announcement failed: {response.json()}")
            return False
    except Exception as e:
        print(f"✗ Station announcement error: {e}")
        return False

def test_safety_announcement():
    """Test safety announcement API"""
    print("\nTesting Safety Announcement...")
    try:
        response = requests.post(
            f"{BASE_URL}/api/announce/safety",
            headers={'X-API-Key': API_KEY},
            json={'language': 'english'}
        )
        if response.ok:
            data = response.json()
            print(f"✓ Safety announcement triggered: {data['message']}")
            return True
        else:
            print(f"✗ Safety announcement failed: {response.json()}")
            return False
    except Exception as e:
        print(f"✗ Safety announcement error: {e}")
        return False

def test_volume_control():
    """Test volume control API"""
    print("\nTesting Volume Control...")
    try:
        # Get current volume
        response = requests.get(
            f"{BASE_URL}/api/audio/volume",
            headers={'X-API-Key': API_KEY}
        )
        if response.ok:
            current_volume = response.json()['volume_percent']
            print(f"✓ Current volume: {current_volume}%")
            
            # Set new volume
            new_volume = 50
            response = requests.post(
                f"{BASE_URL}/api/audio/volume",
                headers={'X-API-Key': API_KEY},
                json={'volume': new_volume}
            )
            if response.ok:
                data = response.json()
                print(f"✓ Volume set to: {data['volume_percent']}%")
                
                # Restore original volume
                requests.post(
                    f"{BASE_URL}/api/audio/volume",
                    headers={'X-API-Key': API_KEY},
                    json={'volume': current_volume}
                )
                return True
            else:
                print(f"✗ Volume set failed: {response.json()}")
                return False
        else:
            print(f"✗ Volume get failed: {response.json()}")
            return False
    except Exception as e:
        print(f"✗ Volume control error: {e}")
        return False

def test_config_api():
    """Test configuration API"""
    print("\nTesting Configuration API...")
    try:
        response = requests.get(
            f"{BASE_URL}/api/config",
            headers={'X-API-Key': API_KEY}
        )
        if response.ok:
            data = response.json()
            print(f"✓ Found {len(data['trains'])} trains")
            print(f"✓ Found {len(data['destinations'])} destinations")
            print(f"✓ Found {len(data['safety_languages'])} safety languages")
            return True
        else:
            print(f"✗ Config API failed: {response.json()}")
            return False
    except Exception as e:
        print(f"✗ Config API error: {e}")
        return False

def test_invalid_api_key():
    """Test API key validation"""
    print("\nTesting API Key Validation...")
    try:
        response = requests.post(
            f"{BASE_URL}/api/announce/safety",
            headers={'X-API-Key': 'invalid-key'},
            json={'language': 'english'}
        )
        if response.status_code == 401:
            print("✓ Invalid API key correctly rejected")
            return True
        else:
            print(f"✗ Invalid API key not rejected: {response.status_code}")
            return False
    except Exception as e:
        print(f"✗ API key test error: {e}")
        return False

def main():
    print("=" * 50)
    print("TARR Annunciator API Test Suite")
    print("=" * 50)
    
    tests = [
        test_api_status,
        test_invalid_api_key,
        test_config_api,
        test_volume_control,
        test_safety_announcement,
        test_station_announcement
    ]
    
    passed = 0
    total = len(tests)
    
    for test in tests:
        if test():
            passed += 1
        time.sleep(1)  # Small delay between tests
    
    print("\n" + "=" * 50)
    print(f"Test Results: {passed}/{total} tests passed")
    print("=" * 50)
    
    if passed == total:
        print("🎉 All tests passed! API is working correctly.")
    else:
        print(f"⚠️  {total - passed} test(s) failed. Check the errors above.")
        
    print(f"\nAPI Documentation: {BASE_URL}/api/docs")
    print(f"API Key: {API_KEY}")

if __name__ == '__main__':
    main()
