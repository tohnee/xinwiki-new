FROM python:3.11-slim

WORKDIR /app

# Create non-root user for runtime security
RUN useradd -m -u 1000 -s /bin/bash mcpuser

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
RUN pip install --no-cache-dir -e . && \
    chown -R mcpuser:mcpuser /app

ENV MCP_HOST=0.0.0.0
ENV MCP_PORT=8000
ENV XINWIKI_BASE_URL=http://app:8080/api/v1

EXPOSE 8000

USER mcpuser

CMD ["xinwiki-mcp-server", "--transport", "http", "--host", "0.0.0.0", "--port", "8000"]
