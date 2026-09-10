"""Stub OCR microservice for LENA2 recipe OCR import (p0).

The real OCR implementation (Tesseract/PaddleOCR/docTR) will replace the
stub response in p1. This endpoint matches the contract expected by
internal/platform/ocrclient.Client.
"""

from __future__ import annotations

from fastapi import FastAPI, File, UploadFile
from fastapi.responses import JSONResponse

app = FastAPI(title="LENA2 OCR", version="0.0.1")


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


@app.post("/ocr")
async def ocr(image: UploadFile = File(...)) -> dict:
    _ = await image.read()
    return {
        "text": "OCR not yet implemented (p0 stub)",
        "lines": [],
    }
