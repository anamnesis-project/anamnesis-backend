from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from typing import List, cast
import queue
import json
from vosk import Model, KaldiRecognizer

app = FastAPI()
stt_model = Model(lang="en-us", model_path="./model/vosk-model-en-us-0.22")


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
            await manager.send_message(f"You wrote: {data}", websocket)
    except WebSocketDisconnect:
        manager.disconnect(websocket)


@app.websocket("/ws/stt")
async def websocket_stt(websocket: WebSocket):
    q = queue.Queue()
    listening = False
    sample_rate = 0
    rec = None

    await manager.connect(websocket)
    try:
        while True:
            msg = await websocket.receive()
            if msg["type"] != "websocket.receive":
                continue

            if "text" in msg:
                msgText = cast(str, msg["text"])
                try:
                    msgJson = json.loads(msgText)
                    if msgJson["command"] == "start":
                        listening = True
                        sample_rate = msgJson["sample_rate"]
                        rec = KaldiRecognizer(stt_model, sample_rate)
                        continue
                    elif msgJson["command"] == "stop":
                        listening = False

                except:
                    await websocket.send_text("invalid json")
                    listening = False
                    q.queue.clear()
                    sample_rate = 0
                    continue
                
            elif "bytes" in msg:
                if not listening:
                    continue

                q.put(msg["bytes"])
                if rec is None:
                    continue

                if rec.AcceptWaveform(msg["bytes"]):
                    await manager.send_message(rec.Result(), websocket)
                else:
                    print(rec.PartialResult())

            await manager.send_message(f"You wrote: {msg}", websocket)
    except WebSocketDisconnect:
        manager.disconnect(websocket)
