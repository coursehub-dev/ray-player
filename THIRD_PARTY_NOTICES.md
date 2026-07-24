# Third-party notices

Ray Player распространяется под GNU GPLv3, но сторонние компоненты и модели сохраняют собственные лицензии и условия распространения.

Основные внешние компоненты:

- Wails — desktop framework;
- ONNX Runtime — inference runtime Microsoft;
- Essentia и Essentia model zoo — аудиоанализ и обученные модели Music Technology Group, Universitat Pompeu Fabra;
- FFmpeg/ffprobe — обработка и декодирование медиа;
- `sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2` — текстовые эмбеддинги;
- Svelte, Vite, Biome и npm-зависимости frontend.

Перед публикацией бинарного релиза сопровождающий обязан проверить лицензии конкретных версий, сохранить notices из распространяемых архивов и не считать GPLv3 проекта автоматической лицензией на веса моделей или системные runtime-библиотеки.
