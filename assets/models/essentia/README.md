1. Перейдите в их репозиторий моделей ([essentia.upf.edu/models.html](https://essentia.upf.edu/models.html)).

> Например: https://essentia.upf.edu/models/classification-heads/mood_happy/

2. Скачайте нужные модели (например, `danceability-vggish-1.pb` или новые версии в формате `.onnx`). 

---

Если модель скачивается в формате TensorFlow (`.pb`), она в одну команду в консоли переводится в `.onnx` через питоновскую утилиту `tf2onnx`:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert --graphdef ray-player1/assets/models/essentia/audioset-vggish-3.pb --output ray-player1/assets/models/essentia/audioset-vggish-3.onnx --inputs model/Placeholder:0 --outputs model/vggish/embeddings:0
```

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert --graphdef ray-player1/assets/models/essentia/danceability-audioset-vggish-1.pb --output ray-player1/assets/models/essentia/danceability-audioset-vggish-1.onnx --inputs model/Placeholder:0 --outputs model/Softmax:0
```

# Справка 

Разница в параметре `--outputs`

В предыдущей команде было: `--outputs model/vggish/embeddings:0`  
В следующей команде стало: `--outputs model/Softmax:0`

Эти две модели (`vggish-3` и `danceability-1`) построены на одной базе (VGGish от Google), но решают разные задачи, поэтому у них разные «выходы»:

1. **Прошлая модель (`embeddings:0`)** — это экстрактор признаков. Она не дает готового ответа (например, «это рок» или «это поп»). На выходе она выдает **эмбеддинг** — сжатый вектор чисел (обычно размером 128), который описывает математический «отпечаток» звука. Этот вектор потом используют другие нейросети для поиска похожих треков или классификации.
2. **Текущая модель (`Softmax:0`)** — это уже конечный классификатор. Слой `Softmax` в нейросетях всегда отвечает за **вероятности**. Данная модель оценивает «танцевальность» (danceability) трека. На выходе из узла `Softmax:0` вы получите конкретные числа — вероятности (например, `[0.15, 0.85]`), которые означают, насколько музыка пригодна для танцев.

Что делает команда по шагам

1. **`uv run --with tf2onnx --with tensorflow`** — временно создает изолированное окружение, скачивает туда нужные версии `tensorflow` и `tf2onnx` и запускает утилиту.
2. **`python -m tf2onnx.convert`** — вызывает модуль конвертера.
3. **`--graphdef ...danceability-audioset-vggish-1.pb`** — берет исходный замороженный граф TensorFlow модели оценки танцевальности.
4. **`--output ...danceability-audioset-vggish-1.onnx`** — указывает имя финального файла, куда запишется готовая ONNX-модель.
5. **`--inputs model/Placeholder:0`** — сообщает конвертеру: «Вход модели находится здесь, сюда мы будем подавать аудио-спектрограмму».
6. **`--outputs model/Softmax:0`** — сообщает конвертеру: «Отрежь всё, что идет после слоя Softmax. Нам нужны финальные предсказания вероятностей на выходе».

---

Как точно узнать input/output names

```bash
uv run --with tensorflow python - <<'PY'
import tensorflow as tf

pb = "ray-player1/assets/models/essentia/tempocnn/deeptemp-k4-3.pb"

with tf.io.gfile.GFile(pb, "rb") as f:
    graph_def = tf.compat.v1.GraphDef()
    graph_def.ParseFromString(f.read())

for n in graph_def.node:
    if n.op == "Placeholder":
        print("INPUT", n.name, n.attr.get("shape"))

print("--- possible outputs ---")
for n in graph_def.node[-30:]:
    print(n.name, n.op)
PY
```

Если увидим следующее:

```json
INPUT input shape {
  dim { size: -1 }
  dim { size: 40 }
  dim { size: -1 }
  dim { size: 1 }
}
```

То правильный формат inputs будет `input:0[1,40,256,1]`

где:

```
1   = batch
40  = mel bands
256 = tempo patch frames
1   = channel
```

Если output не output, а например model/Softmax, тогда команда будет:

```bash
--outputs 'model/Softmax:0'
```

Проверка ONNX после конвертации:

```bash
uv run --with onnx python - <<'PY'
import onnx

model = onnx.load("ray-player1/assets/models/essentia/tempocnn/deeptemp-k4-3.onnx")
onnx.checker.check_model(model)

print("inputs:")
for i in model.graph.input:
    print(i.name, i.type)

print("outputs:")
for o in model.graph.output:
    print(o.name, o.type)
PY
```

---

У большинства есть onnx версия, кроме genre_discogs400-discogs-effnet. На основе метаданных (в ней используется Sigmoid, а не Softmax) вот этой командой подготавливаем onnx модель:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/genre_discogs400-discogs-effnet-1.pb \
  --output ray-player1/assets/models/essentia/genre_discogs400-discogs-effnet-1.onnx \
  --inputs 'serving_default_model_Placeholder:0[-1,1280]' \
  --outputs 'PartitionedCall:0' \
  --opset 13
```

А также конвертируем discogs-effnet-bs64-1:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/discogs-effnet-bs64-1.pb \
  --output ray-player1/assets/models/essentia/discogs-effnet-bs64-1.onnx \
  --inputs 'serving_default_melspectrogram:0[64,128,96]' \
  --outputs 'PartitionedCall:0,PartitionedCall:1' \
  --opset 13
```

А также конвертируем deeptemp-k4-3.pb:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/deeptemp-k4-3.pb \
  --output ray-player1/assets/models/essentia/deeptemp-k4-3.onnx \
  --inputs 'input:0[1,40,256,1]' \
  --outputs 'output:0' \
  --opset 13
```

---


У Essentia в документации для каждой модели прописаны точные имена входных и выходных тензоров.
