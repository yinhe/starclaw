import os, glob

fixes = {
    r'e:\starclaw\api\internal\api\v1\billing.go': {
        3626: 0xA5,   # \xc2\xa5 = yen sign
        3640: 0xA5,   # \xc2\xa5 = yen sign
    },
    r'e:\starclaw\api\internal\api\v1\builtin_agents.go': {
        6516: 0x97,   # \xc3\x97 = multiplication sign
        6536: 0x97,   # \xc3\x97 = multiplication sign
    },
    r'e:\starclaw\api\internal\tool\media_utils.go': {
        10432: 0xB7,  # \xc2\xb7 = middle dot
    },
}

for path, positions in fixes.items():
    with open(path, 'rb') as f:
        data = bytearray(f.read())
    for pos, byte_val in positions.items():
        old = data[pos+1]
        data[pos+1] = byte_val
        ch = bytes([data[pos], byte_val]).decode('utf-8')
        print(f"{os.path.basename(path)} pos {pos}: 0x{old:02x} -> 0x{byte_val:02x} = '{ch}'")
    with open(path, 'wb') as f:
        f.write(data)

# Final verification of ALL Go files
print("\n--- Final Verification ---")
bad = 0
for f in glob.glob(r'e:\starclaw\api\**\*.go', recursive=True):
    with open(f, 'rb') as fh:
        d = fh.read()
    try:
        d.decode('utf-8')
    except UnicodeDecodeError as e:
        bad += 1
        rel = os.path.relpath(f, r'e:\starclaw\api')
        print(f"BROKEN: {rel}: {e}")
if bad == 0:
    print("ALL FILES VALID UTF-8!")
else:
    print(f"{bad} files still broken")
