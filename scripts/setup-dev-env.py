#!/usr/bin/env python3
"""Create .env from .env.example with dev values for local testing."""
import pathlib

p = pathlib.Path('.env')
if not p.exists():
    p.write_text(pathlib.Path('.env.example').read_text())

content = p.read_text()
content = content.replace('# TENANT_AES_KEY=', 'TENANT_AES_KEY=test-tenant-key-16b')
content = content.replace('# SYSTEM_AES_KEY=', 'SYSTEM_AES_KEY=test-system-aes-key-32bytes!!')
content = content.replace('# JWT_SECRET=', 'JWT_SECRET=test-jwt-secret-at-least-32-chars')
p.write_text(content)
print('OK: .env created with dev values')
