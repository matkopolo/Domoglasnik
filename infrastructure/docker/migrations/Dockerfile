FROM python:3.11-slim
RUN pip install --no-cache-dir psycopg[binary]==3.2.9
WORKDIR /database
COPY database/migrations ./migrations
COPY database/production-migrations ./production-migrations
COPY database/production_migrate.py ./
USER 10001:10001
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
ENTRYPOINT ["python", "/database/production_migrate.py"]
