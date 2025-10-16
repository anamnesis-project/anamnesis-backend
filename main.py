from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from tts import TextToSpeech
from stt import SpeechToText
from typing import List
import asyncio

app = FastAPI()
TTSEngine = TextToSpeech()
STTEngine = SpeechToText()

FINAL_SENTENCE_TIMEOUT = 1.5

@app.get("/")
def root():
    return {"message": "Hello from REST API"}

class ConnectionManager:
    def __init__(self):
        self.active_connections: List[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        self.active_connections.remove(websocket)

    async def send_message(self, message: str, websocket: WebSocket):
        await websocket.send_text(message)


manager = ConnectionManager()


@app.websocket("/ws/tts")
async def websocket_tts(websocket: WebSocket):
    await manager.connect(websocket)
    try:
        while True:
            data = await websocket.receive_text()
            print(data)
            audio_bytes = TTSEngine.synthesize_to_bytes(data)
            await websocket.send_bytes(audio_bytes)
            await manager.send_message(f"You wrote: {data}", websocket)
    except WebSocketDisconnect:
        manager.disconnect(websocket)

@app.websocket("/ws/stt")
async def websocket_stt(websocket: WebSocket):
    await manager.connect(websocket)
    recognizer = STTEngine.create_recognizer()
    if not recognizer:
        return

    sentence_parts = []
    timeout_task = None
    print("\STT Client conected. Waiting for message...")

    async def send_final_sentence():
        nonlocal sentence_parts
        if sentence_parts:
            final_sentence = " ".join(sentence_parts)
            print(f"STT Final sentence: '{final_sentence}'")
            await manager.send_message(final_sentence, websocket)
            sentence_parts = []

    try:
        while True:
            audio_chunk = await websocket.receive_bytes()
            print(f"SERVER got {len(audio_chunk)} bytes of audio.", end='\r')

            if recognizer.AcceptWaveform(audio_chunk):
                result_json = recognizer.Result()
                partial_text = STTEngine.process_final_result(result_json)
                
                print(f"\STT partial: '{partial_text}'")
                
                if partial_text:
                    sentence_parts.append(partial_text)

                    if timeout_task:
                        timeout_task.cancel()

                    timeout_task = asyncio.create_task(asyncio.sleep(FINAL_SENTENCE_TIMEOUT))
                    
                    try:
                        await timeout_task
                        await send_final_sentence()
                    except asyncio.CancelledError:
                        continue
    except WebSocketDisconnect:
        await send_final_sentence()
        print("\nSTT client disconected.")
        manager.disconnect(websocket)
    except Exception as e:
        print(f"\nError: STT conection failed: {e}")
        manager.disconnect(websocket)