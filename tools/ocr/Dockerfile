# syntax=docker/dockerfile:1
FROM python:3.14-slim@sha256:caaf356f40667c496d405780745b9ac25771c189a51dfcc42430d531ea09f8a2

WORKDIR /app

# Tesseract OCR, poppler (for pdf2image), and curl for the healthcheck.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        tesseract-ocr \
        tesseract-ocr-eng \
        poppler-utils \
        curl && \
    rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Run as an unprivileged user: the service parses untrusted uploads with
# poppler/Pillow, so it must not run as root.
RUN useradd --create-home --uid 10001 ocr && \
    mkdir -p /tmp && chmod 1777 /tmp
USER ocr

EXPOSE 8000

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
