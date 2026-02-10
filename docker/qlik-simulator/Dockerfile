FROM python:3.11-slim

WORKDIR /app

# Install dependencies
RUN pip install --no-cache-dir \
    confluent-kafka==2.3.0 \
    faker==22.0.0 \
    requests==2.31.0

COPY simulator.py .

ENTRYPOINT ["python", "-u", "simulator.py"]
