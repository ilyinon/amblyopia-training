import wave
import struct
import math

framerate = 44100
duration = 0.15
frequency = 800

wav_file = wave.open("beep.wav", "w")
wav_file.setparams((1, 2, framerate, 0, "NONE", "not compressed"))

for i in range(int(duration * framerate)):
    value = int(32767 * math.sin(2 * math.pi * frequency * i / framerate))
    data = struct.pack('<h', value)
    wav_file.writeframesraw(data)

wav_file.close()
