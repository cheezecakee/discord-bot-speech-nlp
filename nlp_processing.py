import spacy
import sys

nlp = spacy.load("en_core_web_sm")


def extract_song_request(text):
    doc = nlp(text)
    for token in doc:
        if token.lemma_ == "play":
            song = text.replace(token.text, "").strip()
            return song
    return None


if __name__ == "__main__":
    transcript = sys.argv[1]
    song_request = extract_song_request(transcript)
    print(song_request)
