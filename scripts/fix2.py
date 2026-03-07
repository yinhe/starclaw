import os

files = [
    r'e:\starclaw\api\internal\api\v1\billing.go',
    r'e:\starclaw\api\internal\api\v1\builtin_agents.go',
    r'e:\starclaw\api\internal\tool\media_utils.go',
]

for path in files:
    with open(path, 'rb') as f:
        data = f.read()
    
    fname = os.path.basename(path)
    i = 0
    while i < len(data):
        b = data[i]
        if 0xC0 <= b <= 0xDF and i+1 < len(data) and data[i+1] == 0x3F:
            # Show wide context
            start = max(0, i-30)
            end = min(len(data), i+30)
            ctx_before = data[start:i]
            ctx_after = data[i+2:end]
            print(f"\n{fname} pos {i}:")
            print(f"  lead byte: 0x{b:02x}")
            print(f"  bytes around: {' '.join(f'{x:02x}' for x in data[i-5:i+10])}")
            
            # Try to decode context
            try:
                before_text = ctx_before.decode('utf-8', errors='replace')
            except:
                before_text = repr(ctx_before)
            try:
                after_text = ctx_after.decode('utf-8', errors='replace')
            except:
                after_text = repr(ctx_after)
            print(f"  before: ...{before_text}")
            print(f"  after:  {after_text}...")
            
            # Show all possible characters
            print(f"  possible chars for 0x{b:02x} + [80-BF]:")
            for trial in range(0x80, 0xC0):
                try:
                    ch = bytes([b, trial]).decode('utf-8')
                    if ch in '\xa5\xb7\xa0\xab\xbb\xb1\xb0\xd7\xf7':
                        print(f"    0x{trial:02x} = '{ch}' (U+{ord(ch):04X}) <-- likely")
                    elif ord(ch) < 0x100:
                        pass  # skip uncommon latin chars
                except:
                    pass
            # For billing context with %.0f, likely yen sign
            print(f"  ALL options: ", end="")
            for trial in range(0x80, 0xC0):
                try:
                    ch = bytes([b, trial]).decode('utf-8')
                    print(f"'{ch}'", end=" ")
                except:
                    pass
            print()
            i += 2
            continue
        i += 1
