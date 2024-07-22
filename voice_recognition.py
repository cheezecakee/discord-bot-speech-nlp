import vosk
import pyaudio
import json
import sys


def recognize_speech_from_microphone():
    model_path = "./vosk-model-small-en-us-0.15"

    model = vosk.Model(model_path)
    recognizer = vosk.KaldiRecognizer(model, 16000)

    audio = pyaudio.PyAudio()
    stream = audio.open(
        format=pyaudio.paInt16,
        channels=1,
        rate=16000,
        input=True,
        frames_per_buffer=4096,
    )

    print("Listening to command...")

    while True:
        data = sys.stdin.buffer.read(4096)
        if not data:
            ("No data received.")
            break

        print(f"Received data: {len(data)} bytes")  # Log received data length

        if recognizer.AcceptWaveform(data):
            result = json.loads(recognizer.Result())
            print("Recognizer text:", result.get("text", ""))
        else:
            partial_result = json.loads(recognizer.PartialResult())
            print("Partial result:", partial_result.get("partial", ""))


if __name__ == "__main__":
    recognize_speech_from_microphone()
