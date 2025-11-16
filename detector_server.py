from fastapi import FastAPI, UploadFile, File
from fastapi.responses import JSONResponse
from PIL import Image
import io
import uvicorn
from ultralytics import YOLO

# Load model once
model = YOLO("yolov8n.pt")  # Replace with your desired model

app = FastAPI()

@app.post("/detect")
async def detect(file: UploadFile = File(...), conf: float = 0.25, iou: float = 0.45):
    content = await file.read()
    img = Image.open(io.BytesIO(content)).convert("RGB")
    results = model.predict(img, conf=conf, iou=iou, verbose=False)

    boxes = []
    if results:
        r = results[0]
        names = r.names
        for b in r.boxes:
            x1, y1, x2, y2 = map(lambda v: int(v.item()), b.xyxy[0])
            cls_id = int(b.cls.item())
            conf_v = float(b.conf.item())
            boxes.append({
                "label": names.get(cls_id, str(cls_id)),
                "class_id": cls_id,
                "conf": conf_v,
                "x1": x1, "y1": y1, "x2": x2, "y2": y2
            })
    return JSONResponse({"boxes": boxes})

if __name__ == "__main__":
    uvicorn.run(app, host="127.0.0.1", port=9000)