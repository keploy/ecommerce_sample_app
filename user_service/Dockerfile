FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
RUN chmod +x /app/entrypoint.sh
ENV FLASK_RUN_HOST=0.0.0.0
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]
