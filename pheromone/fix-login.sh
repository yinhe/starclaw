#!/bin/bash
FILE=/opt/starclaw/web/src/pages/LoginPage.tsx

# Line 285: title always shows '登录'
sed -i "285s/deployMode === 'opensource'/true/" "$FILE"

# Line 294: main branch — treat hosted same as opensource
sed -i "294s/deployMode === 'opensource'/deployMode !== null/" "$FILE"

echo "Patched LoginPage.tsx"
grep -n "deployMode !== null\|true \?" "$FILE" | head -5
