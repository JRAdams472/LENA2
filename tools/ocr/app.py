"""Local OCR microservice for LENA2 recipe and nutrition OCR.

Supports PNG/JPG images and multi-page PDFs. Uses Tesseract 5.x via
pytesseract and returns extracted text, per-line confidence, and bounding
boxes. PDFs are rasterised with pdf2image + poppler.
"""

from __future__ import annotations

import asyncio
import io
import os
import tempfile
from typing import Annotated, Any

import pytesseract
from fastapi import FastAPI, File, UploadFile
from fastapi.responses import JSONResponse
from pdf2image import convert_from_path, pdfinfo_from_path
from pdf2image.exceptions import PDFInfoNotInstalledError
from PIL import Image, ImageOps
from pytesseract import Output

app = FastAPI(title="LENA2 OCR", version="0.1.0")

# Tesseract page-segmentation modes used by the importer for different layouts.
# Default is --psm 4 (assume single column of variable text). The caller can
# override per-request, but the importer is expected to pick sensible defaults.
DEFAULT_PSM = 4
# Tesseract accepts --psm values 0 through 13 only.
MIN_PSM, MAX_PSM = 0, 13

# Untrusted-upload bounds. The Go BFF enforces the same limits, but the
# sidecar defends itself too since it parses raw bytes with poppler/Pillow.
MAX_UPLOAD_BYTES = int(os.environ.get("OCR_MAX_UPLOAD_BYTES", str(25 * 1024 * 1024)))
MAX_PDF_PAGES = int(os.environ.get("OCR_MAX_PDF_PAGES", "20"))
# Reject decompression bombs larger than ~40 MP (Pillow warns at half this).
Image.MAX_IMAGE_PIXELS = int(os.environ.get("OCR_MAX_IMAGE_PIXELS", str(40_000_000)))
# Bound each rasterized PDF page to the same ~40 MP ceiling: poppler scales
# oversized pages down to fit a MAX_RASTER_DIM square, preserving aspect.
MAX_RASTER_DIM = int(os.environ.get("OCR_MAX_RASTER_DIM", str(int(Image.MAX_IMAGE_PIXELS ** 0.5))))


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


def _write_temp(contents: bytes, suffix: str) -> str:
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
        tmp.write(contents)
        return tmp.name


class _PdfTooManyPages(Exception):
    """Raised by _ocr_pdf so the handler can map it to a 413."""


def _ocr_pdf(path: str, psm: int) -> list[dict[str, Any]]:
    """CPU-bound PDF pipeline: pdfinfo -> bounded rasterize -> per-page OCR.

    Runs inside asyncio.to_thread so poppler and Tesseract never block the
    event loop (and /health stays responsive) during long imports.
    """
    info = pdfinfo_from_path(path)
    page_count = int(info.get("Pages", 0))
    if page_count > MAX_PDF_PAGES:
        raise _PdfTooManyPages(
            f"pdf has {page_count} pages; maximum is {MAX_PDF_PAGES}"
        )
    # -scale-to bounds every rasterized page inside a MAX_RASTER_DIM square,
    # so a crafted huge page cannot explode memory (decompression-bomb
    # equivalent for the poppler path, which bypasses Image.MAX_IMAGE_PIXELS).
    pages = convert_from_path(path, dpi=300, fmt="png", size=MAX_RASTER_DIM)
    results: list[dict[str, Any]] = []
    for i, page_img in enumerate(pages, start=1):
        result = _ocr_image(page_img, psm)
        result["page"] = i
        results.append(result)
    return results


def _ocr_image_bytes(contents: bytes, psm: int) -> dict[str, Any]:
    """CPU-bound single-image pipeline, run inside asyncio.to_thread."""
    page_img = Image.open(io.BytesIO(contents))
    return _ocr_image(page_img, psm)


@app.post("/ocr", response_model=None)
async def ocr(
    image: Annotated[UploadFile, File()], psm: int = DEFAULT_PSM
) -> dict | JSONResponse:
    if not MIN_PSM <= psm <= MAX_PSM:
        return JSONResponse(
            {"error": f"invalid psm {psm}; must be between {MIN_PSM} and {MAX_PSM}"},
            status_code=400,
        )

    # Streamed size limit: read at most MAX_UPLOAD_BYTES + 1 so a huge body
    # is rejected without being fully buffered.
    contents = await image.read(MAX_UPLOAD_BYTES + 1)
    if not contents:
        return JSONResponse({"error": "empty file"}, status_code=400)
    if len(contents) > MAX_UPLOAD_BYTES:
        return JSONResponse(
            {"error": f"file exceeds {MAX_UPLOAD_BYTES} byte limit"},
            status_code=413,
        )

    page_results: list[dict[str, Any]] = []

    if _is_pdf(image):
        # pdf2image needs a file path; write the uploaded PDF to a temp
        # file off the event loop (sync file API in an async handler).
        suffix = os.path.splitext(image.filename or "upload.pdf")[1] or ".pdf"
        tmp_path = await asyncio.to_thread(_write_temp, contents, suffix)
        try:
            page_results = await asyncio.to_thread(_ocr_pdf, tmp_path, psm)
        except _PdfTooManyPages as exc:
            return JSONResponse({"error": str(exc)}, status_code=413)
        except PDFInfoNotInstalledError:
            raise  # server misconfiguration, not a bad upload
        except Exception:
            return JSONResponse({"error": "invalid or unreadable pdf"}, status_code=400)
        finally:
            os.unlink(tmp_path)
    else:
        try:
            result = await asyncio.to_thread(_ocr_image_bytes, contents, psm)
        except Image.DecompressionBombError:
            return JSONResponse({"error": "image exceeds pixel limit"}, status_code=413)
        except Exception:
            return JSONResponse({"error": "invalid or unreadable image"}, status_code=400)
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
