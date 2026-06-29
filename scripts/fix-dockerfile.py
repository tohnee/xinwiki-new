#!/usr/bin/env python3
"""Fix Dockerfile.app: move COPY --from out of RUN command."""
import pathlib

p = pathlib.Path('docker/Dockerfile.app')
content = p.read_text()

old = """    python3 -m pip install --break-system-packages --upgrade pip setuptools wheel && \\
    mkdir -p /home/appuser/.local/bin && \\
    COPY --from=ghcr.io/astral-sh/uv:0.6.6 /uv /home/appuser/.local/bin/uv && \\
    ln -sf /home/appuser/.local/bin/uv /usr/local/bin/uvx && \\
    chown -R appuser:appuser /home/appuser && \\
    chmod +x /usr/local/bin/uvx && \\
    apt-get clean && \\
    rm -rf /var/lib/apt/lists/*"""

new = """    python3 -m pip install --break-system-packages --upgrade pip setuptools wheel && \\
    apt-get clean && \\
    rm -rf /var/lib/apt/lists/*

# Install uv from official image (avoids curl|sh supply chain risk)
COPY --from=ghcr.io/astral-sh/uv:0.6.6 /uv /home/appuser/.local/bin/uv
RUN ln -sf /home/appuser/.local/bin/uv /usr/local/bin/uvx && \\
    chown -R appuser:appuser /home/appuser && \\
    chmod +x /usr/local/bin/uvx"""

if old in content:
    content = content.replace(old, new)
    p.write_text(content)
    print('OK: fixed COPY --from placement')
else:
    print('SKIP: pattern not found')
    # Show context for debugging
    for i, line in enumerate(content.splitlines(), 1):
        if 'COPY --from' in line or 'astral' in line:
            print(f'  Line {i}: {line}')
