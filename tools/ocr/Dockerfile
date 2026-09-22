FROM python:3.12-slim

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
