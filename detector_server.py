from fastapi import FastAPI, UploadFile, File
from fastapi.responses import JSONResponse
from PIL import Image
import io
import os
import platform
import uvicorn

# Optional backends:
# - torch (CPU or MPS on macOS)
# - openvino (recommended on Intel CPU Linux)
USE_BACKEND = os.getenv("BACKEND", "auto").lower()  # auto|torch|openvino

# Common settings
MODEL_PATH = os.getenv("YOLO_MODEL", "yolov8n.pt")
IMG_SIZE = int(os.getenv("YOLO_IMGSZ", "640"))
CONF_DEFAULT = float(os.getenv("CONF_THRESHOLD", "0.25"))
IOU_DEFAULT = float(os.getenv("IOU_THRESHOLD", "0.45"))

app = FastAPI()

def backend_is_openvino():
    return USE_BACKEND == "openvino" or (USE_BACKEND == "auto" and platform.system() == "Linux")

def load_model():
    if backend_is_openvino():
        # Try OpenVINO first on Linux Intel
        try:
            from ultralytics import YOLO
            # You can point OPENVINO_MODEL to a pre-exported IR (.xml or dir).
            ov_path = os.getenv("OPENVINO_MODEL", "")
            if ov_path and os.path.exists(ov_path):
                model = YOLO(ov_path)  # load exported OpenVINO model
                return ("openvino", model)
            # Else export on first run (caches to disk)
            print("[detector] OpenVINO backend selected; exporting model (one-time)...")
            base = YOLO(MODEL_PATH)
            res = base.export(format="openvino", imgsz=IMG_SIZE, dynamic=False)
            # res returns exported path; load it
            model = YOLO(str(res))
            print(f"[detector] OpenVINO model loaded: {res}")
            return ("openvino", model)
        except Exception as e:
            print(f"[detector] OpenVINO init failed ({e}), falling back to torch CPU")

    # Torch (CPU or MPS on macOS)
    import torch
    from ultralytics import YOLO

    device = "cpu"
    half = False
    if platform.system() == "Darwin":
        if torch.backends.mps.is_available():
            device = "mps"
            half = False  # FP16 not supported on MPS reliably
    # Else Linux Intel CPU -> device=cpu

    try:
        # Enable better CPU perf on stable sizes
        try:
            torch.set_num_threads(max(1, int(os.getenv("TORCH_NUM_THREADS", "0"))))
        except Exception:
            pass

        model = YOLO(MODEL_PATH)
        model.to(device)
        return (f"torch:{device}", model)
    except Exception as e:
        raise RuntimeError(f"Failed to load YOLO model with torch: {e}")

BACKEND, MODEL = load_model()

@app.get("/health")
def health():
    info = {
        "backend": BACKEND,
        "model": MODEL_PATH,
        "imgsz": IMG_SIZE,
        "platform": platform.platform(),
    }
    if BACKEND.startswith("torch"):
        try:
            import torch
            info.update({
                "torch_version": torch.__version__,
                "mps_available": torch.backends.mps.is_available() if platform.system()=="Darwin" else False,
            })
        except Exception:
            pass
    return info

@app.post("/detect")
async def detect(file: UploadFile = File(...), conf: float = CONF_DEFAULT, iou: float = IOU_DEFAULT):
    content = await file.read()
    img = Image.open(io.BytesIO(content)).convert("RGB")

    # Ultralytics .predict() works across backends (torch/openvino)
    results = MODEL.predict(
        img,
        conf=conf,
        iou=iou,
        imgsz=IMG_SIZE,
        verbose=False,
    )

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
    port = int(os.getenv("DETECTOR_PORT", "9000"))
    print(f"[detector] backend={BACKEND} imgsz={IMG_SIZE} model={MODEL_PATH}")
    uvicorn.run(app, host="127.0.0.1", port=port)