#!/bin/bash

# Script to convert MP3 files to WAV for better pygame compatibility
# This avoids pygame MP3 decoder issues entirely

echo "Converting MP3 files to WAV format..."

# Find all MP3 files and convert them to WAV
find static/mp3 -name "*.mp3" -type f | while read file; do
    echo "Converting: $file"
    
    # Get the base name without extension
    base_name="${file%.mp3}"
    wav_file="${base_name}.wav"
    
    # Convert to WAV using ffmpeg
    ffmpeg -i "$file" -acodec pcm_s16le -ar 22050 "$wav_file" -y 2>/dev/null
    
    # Check if conversion was successful
    if [ $? -eq 0 ]; then
        echo "✓ Converted: $file -> $wav_file"
        # Optionally remove the original MP3 file
        # rm "$file"
    else
        echo "✗ Failed to convert: $file"
    fi
done

echo "MP3 to WAV conversion complete!"
echo "Note: You'll need to update your JSON config files to reference .wav instead of .mp3"
