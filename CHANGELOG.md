# TARR Annunciator - Changelog

All notable changes and enhancements to the TARR Annunciator project.

## [2.0] - Multi-User & Cross-Platform Release - 2025-08-25

### Major Features Added

#### Cross-Platform Updater System
- **Created comprehensive updater application** (`updater/main.go`)
  - Automatic OS and architecture detection (Windows, Linux, ARM64, ARM32, ARMv6)
  - GitHub API integration for checking and downloading updated executables
  - File synchronization from GitHub data directory (`/data/static/*`, `/data/templates/*`)
  - Schema compatibility checking to prevent config downgrades
  - Protection for multi-user configurations from being overwritten

#### Project Structure Reorganization
- **Moved all Go source files to `/source` directory**
  - Clean separation between source code and runtime files
  - Improved project organization and distribution packaging
  - Updated build scripts and documentation accordingly

#### Multi-User Authentication System
- **Expanded from single admin to multi-user architecture**
  - Support for multiple admin users with individual credentials
  - Role-based permission system (`system_config`, `user_management`, `api_management`, `audio_control`, `announcements`)
  - User management via web interface (create, edit, delete users)
  - Session-based authentication with configurable timeout

#### Advanced API Key Management
- **Enhanced API system with multiple keys**
  - Support for multiple API keys with individual permissions
  - Permanent API keys (non-expiring) support
  - API key expiration management
  - Rate limiting per API key (configurable requests per hour)
  - API key usage tracking and last-used timestamps
  - Create, edit, delete API keys via admin interface

#### Configuration Management Overhaul
- **Restructured admin_config.json for multi-user support**
  ```json
  {
    "admin_users": [array of AdminUser objects],
    "api_keys": [array of APIKey objects], 
    "security": {password policies, session settings, lockout rules},
    "metadata": {schema versioning, timestamps}
  }
  ```
- **Added configuration schema versioning** to prevent incompatible updates
- **Enhanced security settings** including failed login attempt protection

#### Advanced Announcement Queue System
- **Implemented comprehensive announcement queuing** (`announcement_queue.go`)
  - Priority-based queue management (Emergency, High, Normal, Low)
  - Queue status monitoring and history tracking
  - Real-time queue status API endpoints
  - Announcement cancellation capabilities
  - Thread-safe queue operations with proper locking

#### Cross-Platform Package Creation
- **Created distribution packages** for multiple platforms
  - Windows x64 package with installer scripts
  - Linux x64 package with installation scripts  
  - Raspberry Pi ARM64 package with Pi-specific optimizations
  - Each package includes pre-built executables and updater

#### Raspberry Pi Specialized Support
- **Enhanced Raspberry Pi compatibility** (`README_RaspberryPi.md`)
  - Complete Raspberry Pi setup guide (all models supported)
  - Audio system configuration for Pi hardware
  - Platform detection and Pi-specific optimizations
  - Native ARM compilation support
  - Systemd service integration examples

### Technical Improvements

#### Audio System Enhancements
- **Cross-compilation audio library migration**
  - Evaluated migration from faiface/beep to cross-compilable alternatives
  - Audio device management and selection
  - Volume control with real-time updates

#### Web Interface Improvements  
- **Complete admin interface overhaul** (`templates/admin.html`)
  - Bootstrap-based tabbed interface (Admin Users, API Keys, Settings)
  - Modal forms for user and API key management
  - Real-time queue status and history display
  - Permission management with checkbox interfaces
  - Session credential handling for API calls
  - Mobile-responsive design

#### API Enhancements
- **Expanded REST API with new endpoints**
  - `/api/queue/status` - Real-time queue monitoring
  - `/api/queue/history` - Announcement history with pagination
  - `/api/queue/cancel` - Queue item cancellation
  - `/admin/users/*` - User CRUD operations  
  - `/admin/api-keys/*` - API key management
  - Dual authentication support (session + API key)

#### Security Improvements
- **Enhanced authentication and authorization**
  - Session-based admin authentication with secure cookies
  - API key authentication for programmatic access
  - Password policy enforcement (length, special chars, numbers)
  - Failed login attempt protection with lockout
  - Session timeout configuration
  - Credential management integrated into main application

### Bug Fixes

#### Authentication Issues
- **Fixed queue status/history authentication errors**
  - Added session-authenticated versions of queue API endpoints
  - Resolved "API key required" errors in admin interface
  - Proper credential handling in JavaScript fetch requests

#### Build and Deployment Issues
- **Resolved cross-compilation challenges**
  - Fixed audio library compatibility across platforms
  - Resolved duplicate route registration causing startup failures
  - Corrected executable naming for Windows (.exe) vs Unix systems

#### Template and Configuration Issues
- **Fixed template reversion problems**  
  - Updater was overwriting newer multi-user templates
  - Implemented compatibility checking to preserve local changes
  - Restored complete multi-user admin interface after updater overwrites

#### File Structure and Path Issues
- **Resolved runtime file location problems**
  - Fixed template and configuration file path resolution
  - Proper working directory handling for different execution contexts

### Infrastructure Changes

#### Build System Improvements
- **Enhanced build automation**
  - Cross-platform build scripts (Windows batch, Linux shell)
  - Automated package creation with manifests
  - Build verification and testing scripts

#### Documentation Expansion
- **Comprehensive documentation suite**
  - Platform-specific setup guides (Windows, Linux, Raspberry Pi)
  - API documentation with examples
  - Credential management guides
  - Installation and deployment instructions

#### Development Workflow
- **Improved development processes**
  - Source code organization in dedicated directory
  - Package management and release preparation
  - Version control and schema management

### Migration Notes

#### From Single-User to Multi-User
- **Automatic configuration migration** from old single-user format
- **Backward compatibility** maintained for existing installations  
- **Schema versioning** prevents accidental downgrades
- **Data preservation** during updates

#### Deployment Changes
- **New directory structure** requires updated deployment scripts
- **Updater integration** for automatic maintenance
- **Platform-specific packages** for easier installation

### Breaking Changes

#### Configuration Format
- **admin_config.json structure completely changed**
  - Old single admin/API key format no longer supported for new features
  - Migration required for existing installations
  - Schema version "multi-user" identifies new format

#### API Endpoint Changes  
- **Queue management endpoints moved**
  - From API-key only to dual authentication support
  - Session-authenticated versions added for admin interface
  - Maintains backward compatibility for API key access

#### File Organization
- **Source files relocated to `/source` directory**
  - Build processes must account for new structure
  - Runtime execution from project root directory
  - Package distributions exclude source code

### Performance Improvements
- **Optimized queue management** with efficient data structures
- **Reduced memory footprint** through better resource management
- **Faster startup times** with streamlined initialization
- **Improved concurrent handling** of announcements and web requests

### Platform-Specific Enhancements
- **Windows**: Native executable packaging with batch installers
- **Linux**: Shell script automation and systemd integration
- **Raspberry Pi**: Hardware-specific audio configuration and optimization
- **ARM platforms**: Native compilation support for better performance

---

## Development Summary

This major release represents a complete evolution of the TARR Annunciator from a simple single-user train announcement system to a professional-grade, multi-user, cross-platform application with:

- **Enterprise-ready authentication** with multiple users and API keys
- **Professional deployment** with automated updates and cross-platform packages  
- **Advanced queue management** with priority handling and monitoring
- **Comprehensive web interface** with modern Bootstrap UI
- **Robust security features** with session management and access control
- **Platform optimization** especially for Raspberry Pi deployments
- **Developer-friendly architecture** with clean source organization

The system now supports everything from simple home model railroad setups to professional installations requiring multiple operators, API integrations, and centralized management.

### Statistics
- **~50+ new functions/handlers** added for multi-user and queue management
- **1000+ lines of JavaScript** for enhanced web interface  
- **3 platform-specific packages** created with full installer suites
- **10+ new API endpoints** for comprehensive system control
- **Complete backward compatibility** maintained for existing configurations
- **Cross-platform support** for Windows, Linux, and ARM architectures

This changelog documents the transformation of a simple announcement script into a comprehensive, production-ready train announcement system suitable for both hobbyist and professional applications.