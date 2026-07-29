FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

COPY pyproject.toml ./
COPY real_estate_etl ./real_estate_etl
RUN pip install --no-cache-dir ".[ml]" \
    && useradd --create-home --uid 10001 pipeline \
    && mkdir -p /data/raw \
    && chown -R pipeline:pipeline /data

USER pipeline

ENTRYPOINT ["real-estate-etl"]
CMD ["--help"]
