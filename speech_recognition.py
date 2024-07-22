import os
from google.cloud import speech
import sys

# Set up Google Cloud credentials
os.environ["GOOGLE_APPLICATION_CREDENTAILS"] = "path/to/credentials.json"

client = speech.SpeechClient()


def recognize_speech(file_path):
    with open(file_path, "rb") as audio_file:
        content = audio_file.read()

    audio = speech.RecognitionAudio(content=content)
    config = speech.RecognitionConfig(
        encoding=speech.RecognitionConfig.AudioEncoding.LINEAR16,
        sample_rate_hertz=16000,
        language_code="en-US",
    )

    response = client.recognize(config=config, audio=audio)
    for result in response.results:
        return result.alternatives[0].transcript


if __name__ == "__main__":
    audio_file_path = sys.argv[1]
    transcript = recognize_speech(audio_file_path)
    print(transcript)
