# Emergency Announcement Audio Files

This directory contains MP3 audio files for emergency announcements.

## Required Files

Based on the `emergencies.json` configuration, the following audio files should be placed in this directory:

- `fire_evacuation.mp3` - Fire evacuation announcement
- `medical_emergency.mp3` - Medical emergency announcement  
- `security_alert.mp3` - Security alert announcement
- `severe_weather.mp3` - Severe weather warning
- `system_shutdown.mp3` - System shutdown announcement
- `bomb_threat.mp3` - Bomb threat evacuation
- `power_outage.mp3` - Power outage notification
- `chemical_spill.mp3` - Chemical spill emergency
- `lockdown.mp3` - Security lockdown procedure
- `all_clear.mp3` - All clear announcement

## File Format

- **Format**: MP3
- **Sample Rate**: 44.1 kHz recommended
- **Bit Rate**: 128 kbps or higher
- **Channels**: Mono or Stereo

## Usage

Emergency announcements are triggered via:

1. **Admin Interface**: Select emergency type from dropdown and click "🚨 Trigger Emergency"
2. **API**: `POST /api/announce/emergency` with `file` parameter
3. **Highest Priority**: Emergency announcements always have priority 5 and jump to the front of the queue

## Adding New Emergencies

To add new emergency types:

1. Add the new emergency to `json/emergencies.json`
2. Place the corresponding MP3 file in this directory
3. The filename should match the `id` field in the JSON configuration

Example:
```json
{
    "id": "earthquake",
    "name": "Earthquake Alert", 
    "description": "Earthquake emergency procedures",
    "category": "Natural Disaster"
}
```

Would require: `earthquake.mp3` in this directory.