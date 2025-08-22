#!/bin/bash

# Script to fix MP3 files that cause pygame warnings
# This re-encodes all MP3 files to remove extra data

echo "Fixing MP3 files to remove extra data..."

# Find all MP3 files and re-encode them
find static/mp3 -name "*.mp3" -type f | while read file; do
    echo "Processing: $file"
    
    # Create temporary file
    temp_file="${file}.tmp"
    
    # Re-encode with ffmpeg to clean up the file
    ffmpeg -i "$file" -acodec mp3 -ab 128k "$temp_file" -y 2>/dev/null
    
    # Check if re-encoding was successful
    if [ $? -eq 0 ]; then
        # Replace original with cleaned version
        mv "$temp_file" "$file"
        echo "✓ Fixed: $file"
    else
        # Remove temp file if encoding failed
        rm -f "$temp_file"
        echo "✗ Failed to fix: $file"
    fi
done

echo "MP3 file cleanup complete!"
