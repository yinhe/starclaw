#!/usr/bin/env python3
import re, sys

with open('/etc/nginx/sites-enabled/queen') as f:
    content = f.read()

snippet = open('/tmp/nydus-loc.conf').read()

# Insert after ssl_ciphers line in the starclaw.net server block
pattern = r'(server_name starclaw\.net www\.starclaw\.net;.*?ssl_ciphers HIGH:!aNULL:!MD5;\n)'
new_content = re.sub(pattern, r'\1\n' + snippet + '\n', content, count=1, flags=re.DOTALL)

if new_content == content:
    print('WARN: pattern not found, no changes made')
    sys.exit(1)

with open('/etc/nginx/sites-enabled/queen', 'w') as f:
    f.write(new_content)
print('OK')
