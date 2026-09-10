"""Local OCR microservice for LENA2 recipe and nutrition OCR.

Supports PNG/JPG images and multi-page PDFs. Uses Tesseract 5.x via
pytesseract and returns extracted text, per-line confidence, and bounding
boxes. PDFs are rasterised with pdf2image + poppler.
"""

from __future__ import annotations

import io
import os
import tempfile
from typing import Any

import pytesseract
from fastapi import FastAPI, File, UploadFile
from fastapi.responses import JSONResponse
from pdf2image import convert_from_path
from PIL import Image, ImageOps
from pytesseract import Output

app = FastAPI(title="LENA2 OCR", version="0.1.0")

# Tesseract page-segmentation modes used by the importer for different layouts.
# Default is --psm 4 (assume single column of variable text). The caller can
# override per-request, but the importer is expected to pick sensible defaults.
DEFAULT_PSM = 4


@app.get("/health")
def health() -> dict:
    return {"status": "ok"}


def _preprocess(img: Image.Image) -> Image.Image:
    """Basic pre-processing before Tesseract: grayscale and autocontrast."""
    if img.mode not in ("L", "RGB"):
        img = img.convert("RGB")
    gray = img.convert("L")
    # Autocontrast improves text/background separation for scans and photos.
    return ImageOps.autocontrast(gray)


def _ocr_image(img: Image.Image, psm: int) -> dict[str, Any]:
    """Run Tesseract on one PIL image and return text + lines."""
    processed = _preprocess(img)
    config = f"--psm {psm}"
    data = pytesseract.image_to_data(
        processed,
        output_type=Output.DICT,
        config=config,
        lang="eng",
    )

    lines: dict[tuple[int, int, int, int], dict[str, Any]] = {}
    for i, text in enumerate(data["text"]):
        conf = int(data["conf"][i])
        if not text or conf < 0:
            continue
        key = (data["page_num"][i], data["block_num"][i], data["par_num"][i], data["line_num"][i])
        if key not in lines:
            lines[key] = {
                "text": "",
                "conf_sum": 0,
                "conf_count": 0,
                "x1": data["left"][i],
                "y1": data["top"][i],
                "x2": data["left"][i] + data["width"][i],
                "y2": data["top"][i] + data["height"][i],
            }
        line = lines[key]
        if line["text"]:
            line["text"] += " "
        line["text"] += text.strip()
        line["conf_sum"] += conf
        line["conf_count"] += 1
        # Expand bounding box to include all words on the line.
        x1 = data["left"][i]
        y1 = data["top"][i]
        x2 = x1 + data["width"][i]
        y2 = y1 + data["height"][i]
        line["x1"] = min(line["x1"], x1)
        line["y1"] = min(line["y1"], y1)
        line["x2"] = max(line["x2"], x2)
        line["y2"] = max(line["y2"], y2)

    line_list: list[dict[str, Any]] = []
    total_conf, conf_count = 0, 0
    for _, line in sorted(lines.items()):
        if not line["text"]:
            continue
        avg_conf = line["conf_sum"] / max(line["conf_count"], 1)
        total_conf += line["conf_sum"]
        conf_count += line["conf_count"]
        line_list.append({
            "text": line["text"],
            "conf": round(avg_conf, 2),
            "bbox": {
                "x": line["x1"],
                "y": line["y1"],
                "width": line["x2"] - line["x1"],
                "height": line["y2"] - line["y1"],
            },
        })

    full_text = "\n".join(line["text"] for line in line_list)
    page_conf = round(total_conf / max(conf_count, 1), 2)
    return {
        "text": full_text,
        "lines": line_list,
        "mean_confidence": page_conf,
    }


def _is_pdf(file: UploadFile) -> bool:
    name = (file.filename or "").lower()
    return name.endswith(".pdf") or (file.content_type == "application/pdf")


@app.post("/ocr")
async def ocr(image: UploadFile = File(...), psm: int = DEFAULT_PSM) -> dict:
    contents = await image.read()
    if not contents:
        return JSONResponse({"error": "empty file"}, status_code=400)

    page_results: list[dict[str, Any]] = []

    if _is_pdf(image):
        # pdf2image needs a file path; write the uploaded PDF to a temp file.
        suffix = os.path.splitext(image.filename or "upload.pdf")[1] or ".pdf"
        with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
            tmp.write(contents)
            tmp_path = tmp.name
        try:
            pages = convert_from_path(tmp_path, dpi=300, fmt="png")
            for i, page_img in enumerate(pages, start=1):
                result = _ocr_image(page_img, psm)
                result["page"] = i
                page_results.append(result)
        finally:
            os.unlink(tmp_path)
    else:
        page_img = Image.open(io.BytesIO(contents))
        result = _ocr_image(page_img, psm)
        result["page"] = 1
        page_results.append(result)

    all_lines: list[dict[str, Any]] = []
    all_texts: list[str] = []
    for page in page_results:
        all_texts.append(page["text"])
        all_lines.extend(page["lines"])

    return {
        "text": "\n\n".join(all_texts),
        "lines": all_lines,
        "pages": page_results,
    }
