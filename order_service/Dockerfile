FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt ./
# Keploy injects its per-run MITM CA and the trust env vars automatically; no
# certificate needs to be baked in.
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
RUN chmod +x /app/entrypoint.sh
ENV FLASK_RUN_HOST=0.0.0.0
ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]

