# Build frontend
FROM node:22-alpine AS frontend
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json* frontend/
RUN cd frontend && (npm ci 2>/dev/null || npm install)
COPY frontend/ frontend/
RUN cd frontend && npm run build

# Runtime — slim Python image for ARM
FROM python:3.12-slim-bookworm
WORKDIR /app

COPY backend/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY backend/app ./app
COPY --from=frontend /src/backend/static ./static

ENV BYTEBAY_DB=/data/bytebay.db
ENV BYTEBAY_AGENT_SOCKET=/run/bytebay/agent.sock

EXPOSE 8080
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
