import io
from piper.voice import PiperVoice
import wave


TTS_MODEL = "models/en_US-amy-medium.onnx"
TTS_AUDIOFILE = "output.wav"

class TextToSpeech:
    def __init__(self):
        self.model_path = TTS_MODEL
        self.voice = PiperVoice.load(self.model_path)

    def synthesize_to_file(self, text):
        with wave.open(TTS_AUDIOFILE, "wb") as wav_file:
            self.voice.synthesize_wav(text, wav_file)
        return TTS_AUDIOFILE

    def synthesize_to_bytes(self, text):
        buffer = io.BytesIO()
        with wave.open(buffer, "wb") as wav_file:
            self.voice.synthesize_wav(text, wav_file)
        return buffer.getvalue()


if __name__ == "__main__":
    tts = TextToSpeech()

    text = "Hello, this is a test with Piper TTS."

    tts.synthesize_to_file(text)
    varTest = tts.synthesize_to_bytes(text)