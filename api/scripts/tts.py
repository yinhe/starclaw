#!/usr/bin/env python3
"""DashScope TTS script using CosyVoice SDK.
Usage: python3 tts.py <api_key> <text> <voice> <output_path>
Voices: longyuan, longxiaochun, longhua, longjing, longshuo, longshu, longwan, longfei
"""
import sys
import os

def main():
    if len(sys.argv) < 5:
        print("Usage: tts.py <api_key> <text> <voice> <output_path>", file=sys.stderr)
        sys.exit(1)

    api_key = sys.argv[1]
    text = sys.argv[2]
    voice = sys.argv[3]
    output_path = sys.argv[4]

    os.environ["DASHSCOPE_API_KEY"] = api_key
    import dashscope
    dashscope.api_key = api_key

    from dashscope.audio.tts_v2 import SpeechSynthesizer

    model = "cosyvoice-v1"
    
    try:
        synthesizer = SpeechSynthesizer(model=model, voice=voice)
        audio_data = synthesizer.call(text)

        if audio_data and len(audio_data) > 0:
            with open(output_path, "wb") as f:
                f.write(audio_data)
            print(f"OK:{len(audio_data)}", flush=True)
        else:
            print(f"ERROR:empty audio data", file=sys.stderr)
            sys.exit(1)
    except Exception as e:
        print(f"ERROR:{e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
