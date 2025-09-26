from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from tts import TextToSpeech
from typing import List
import queuex   
from vosk import Model, KaldiRecognizer

app = FastAPI()
stt_model = Model(lang="en-us", model_path="./model/vosk-model-en-us-0.22")
TTSEngine = TextToSpeech()

@app.get("/")
def root():
    return {"message": "Hello from REST API"}


# @app.get("/items/{item_id}")
# def read_item(item_id: int, q: str | None = None):
#     return {"item_id": item_id, "q": q}


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
    q = queue.Queue()
    listening = False

    await manager.connect(websocket)
    try:
        while True:
            msg = await websocket.receive()
            if msg["type"] != "websocket.receive":
                continue

            if "text" in msg:
                '''
                    Parse text as json (start or end + sample rate)
                '''
                # if msg == "start":
                #     listening = True
                # elif msg == "end":
                #     listening = False
            elif "bytes" in msg:
                # send bytes to transcriber

            await manager.send_message(f"You wrote: {data}", websocket)
    except WebSocketDisconnect:
        manager.disconnect(websocket)
