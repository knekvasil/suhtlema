# Local, browser-free, single-purpose measurement target

Status: accepted

The MVP runs STT, LLM, and TTS entirely on the development MacBook Air (M1, 8 GB, fanless), which is treated as a single-purpose measurement target: no browser or other memory-heavy application runs on it during measurement. The machine has only ~5–6 GB usable after macOS and the GPU wire limit, and the three resident models consume most of that; a browser would push inference into swap and destroy the very latency we are trying to measure. A browser-based client is out of scope until after the local baseline is recorded.
