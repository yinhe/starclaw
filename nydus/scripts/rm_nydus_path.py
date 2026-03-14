#!/usr/bin/env python3
"""Remove the /nydus/ location block from the queen nginx config."""
import re

path = '/etc/nginx/sites-enabled/queen'
with open(path) as f:
    content = f.read()

# Remove the /nydus/ location block (comment + location block)
pattern = r'\n\s*# Nydus release mirror.*?\n\s*location /nydus/ \{.*?\}\n'
new_content = re.sub(pattern, '\n', content, count=1, flags=re.DOTALL)

if new_content == content:
    print('WARN: /nydus/ block not found')
else:
    with open(path, 'w') as f:
        f.write(new_content)
    print('OK: removed /nydus/ location block')
