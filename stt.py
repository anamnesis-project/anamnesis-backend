import json
from vosk import Model, KaldiRecognizer

STT_MODEL = "models/vosk-model-en-us-0.22"
class SpeechToText:
    def __init__(self, sample_rate: float = 16000.0):
        try:
            self.model = Model(STT_MODEL)
            self.sample_rate = sample_rate
            print("Loaded Vosk model.")
        except Exception as e:
            print(f"ERROR loading Vosk model: {e}")
            self.model = None

    def create_recognizer(self) -> KaldiRecognizer | None:
        if self.model:
            return KaldiRecognizer(self.model, self.sample_rate)
        return None

    def process_final_result(self, result_json: str) -> str:
        result_dict = json.loads(result_json)
        return result_dict.get("text", "")