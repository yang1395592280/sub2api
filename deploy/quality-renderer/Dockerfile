FROM mcr.microsoft.com/playwright/python:v1.64.1-noble
COPY requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
WORKDIR /app
COPY renderer.py /app/renderer.py
USER pwuser
EXPOSE 8091
HEALTHCHECK --interval=30s --timeout=3s CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8091/health', timeout=2)"
CMD ["python", "/app/renderer.py"]
