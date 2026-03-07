import os, glob

files_to_fix = [
    r'e:\starclaw\api\internal\api\v1\billing.go',
    r'e:\starclaw\api\internal\api\v1\builtin_agents.go',
    r'e:\starclaw\api\internal\tool\media_utils.go',
]

for path in files_to_fix:
    with open(path, 'rb') as f:
        data = bytearray(f.read())
    
    fixes = 0
    i = 0
    while i < len(data):
        b = data[i]
        # 2-byte UTF-8 with corrupted 2nd byte
        if 0xC0 <= b <= 0xDF and i+1 < len(data) and data[i+1] == 0x3F:
            next_b = data[i+2] if i+2 < len(data) else None
            candidates = []
            for trial in range(0x80, 0xC0):
                try:
                    bytes([b, trial]).decode('utf-8')
                except:
                    continue
                # Must be INVALID in GBK (otherwise wouldn't have been corrupted)
                try:
                    bytes([b, trial]).decode('gbk')
                    continue  # valid GBK = would not have been corrupted
                except:
                    pass
                candidates.append(trial)
            
            if candidates:
                # Pick the most likely candidate
                chosen = candidates[0]
                ch = bytes([b, chosen]).decode('utf-8')
                data[i+1] = chosen
                fixes += 1
                fname = os.path.basename(path)
                print(f"{fname} pos {i}: 0x{b:02x} 0x3f -> 0x{b:02x} 0x{chosen:02x} = '{ch}' (from {len(candidates)} candidates)")
            i += 2
            continue
        
        # Also catch any remaining 3-byte issues
        if 0xE0 <= b <= 0xEF and i+2 < len(data):
            b2 = data[i+1]
            b3 = data[i+2]
            if 0x80 <= b2 <= 0xBF and b3 == 0x3F:
                next_b = data[i+3] if i+3 < len(data) else None
                candidates = []
                for trial in range(0x80, 0xC0):
                    try:
                        bytes([b, b2, trial]).decode('utf-8')
                    except:
                        continue
                    if next_b is not None:
                        try:
                            bytes([trial, next_b]).decode('gbk')
                            continue
                        except:
                            pass
                    candidates.append(trial)
                if candidates:
                    data[i+2] = candidates[0]
                    fixes += 1
                i += 3
                continue
        i += 1
    
    if fixes > 0:
        with open(path, 'wb') as f:
            f.write(data)
        print(f"  -> Fixed {fixes} in {os.path.basename(path)}")

# Final verification
print("\n--- Verification ---")
for f in glob.glob(r'e:\starclaw\api\**\*.go', recursive=True):
    with open(f, 'rb') as fh:
        data = fh.read()
    try:
        data.decode('utf-8')
    except UnicodeDecodeError as e:
        rel = os.path.relpath(f, r'e:\starclaw\api')
        print(f"STILL BROKEN: {rel}: {e}")
print("Verification complete.")
