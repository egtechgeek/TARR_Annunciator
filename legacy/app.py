from flask import Flask, render_template, request
from pydub import AudioSegment
from pydub.playback import play
import os

app = Flask(__name__)

# Folder path for MP3 files (Train, Direction, Destination, Track, and Safety announcements)
MP3_PATHS = {
    'train': 'static/mp3/train/',  # Path to train MP3 files
    'direction': 'static/mp3/direction/',  # Path to direction MP3 files
    'destination': 'static/mp3/destination/',  # Path to destination MP3 files
    'track': 'static/mp3/track/',  # Path to track MP3 files
    'promo': 'static/mp3/promo/',  # Path to promotional MP3 files
    'english': 'static/mp3/safety/safety_english2.mp3',  # English safety announcement
    'spanish': 'static/mp3/safety/safety_spanish.mp3',  # Spanish safety announcement
    'portuguese': 'static/mp3/safety/safety_portuguese.mp3',  # Portuguese safety announcement
    'russian': 'static/mp3/safety/safety_russian.mp3',  # Russian safety announcement
}

# Variable to track the current safety announcement process
current_announcement = None

# Route to render the HTML page with the dropdown options
@app.route('/')
def index():
    trains = ['1', '9', '27', '347', '415', '428', '573', '774', '815', '2006', '2199', '4900', '944']  # Example train numbers
    directions = ['westbound', 'eastbound']  # Example directions
    destinations = ['goodwin_station', 'tradewinds_central_station', 'picnic_station', 'yard', 'hialeah']  # Example destinations
    tracks = ['1', '2', 'express']  # Example tracks

    return render_template('index.html', trains=trains, directions=directions, destinations=destinations, tracks=tracks)

# Route to handle the promo announcement request
@app.route('/play_promo', methods=['POST'])
def play_promo():
    promo_file = os.path.join(MP3_PATHS['promo'], 'promo_english.mp3')  # Add your promo file
    play_audio(promo_file)  # Make sure play_audio method is defined to handle this
    return 'Promo Announcement Played'

# Route to handle the Station Announcement
@app.route('/play_announcement', methods=['POST'])
def play_announcement():
    train_number = request.form.get('train_number')
    direction = request.form.get('direction')
    destination = request.form.get('destination')
    track_number = request.form.get('track_number')

    # Stop the safety announcement if it's playing
    stop_safety_announcement()

    # Play the corresponding MP3 files for the station announcement
    play_station_announcement(train_number, direction, destination, track_number)
    return 'OK'

# Route to handle the safety announcement language selection
@app.route('/play_safety_announcement', methods=['POST'])
def play_safety_announcement():
    language = request.form.get('language')

    # Stop any current safety announcement if it's playing
    stop_safety_announcement()

    # Play the corresponding language safety announcement
    if language in MP3_PATHS:
        safety_file = MP3_PATHS[language]
        play_audio(safety_file)
        return f"Safety announcement in {language.capitalize()} played."
    else:
        return 'Invalid language selected!', 400

# Function to stop the safety announcement (currently does nothing since it's manual control only)
def stop_safety_announcement():
    global current_announcement
    current_announcement = None  # In case we decide to add stop functionality in the future

# Function to play a station announcement
def play_station_announcement(train_number, direction, destination, track_number):
    chime_file = 'static/mp3/chime.mp3'
    
    # Correct order of MP3 files: Chime, Train, Direction, Destination, Track
    mp3_files = [
        chime_file,
        os.path.join(MP3_PATHS['train'], f"{train_number}.mp3"),
        os.path.join(MP3_PATHS['direction'], f"{direction}.mp3"),
        os.path.join(MP3_PATHS['destination'], f"{destination}.mp3"),
        os.path.join(MP3_PATHS['track'], f"{track_number}.mp3")
    ]
    
    # Combine all MP3 files into one
    combined_audio = AudioSegment.empty()
    for mp3_file in mp3_files:
        try:
            audio = AudioSegment.from_mp3(mp3_file)
            combined_audio += audio  # Concatenate each audio file
        except Exception as e:
            print(f"Error loading file {mp3_file}: {e}")
    
    # Play the combined audio as a single stream
    play(combined_audio)

# Function to play audio using pydub (sequential playback without pauses)
def play_audio(file):
    """ Play audio using pydub """
    try:
        # Load the MP3 file
        audio = AudioSegment.from_mp3(file)
        # Play the audio
        play(audio)  # Play the audio file
    except Exception as e:
        print(f"Error playing audio file {file}: {e}")

if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0', port=8080)
