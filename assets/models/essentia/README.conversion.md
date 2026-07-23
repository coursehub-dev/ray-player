Да, по выводу графа видно, что вы почти верно конвертировали. Нужно только зафиксировать правильные `inputs/outputs` и не трогать `saver_filename`.

---

# 1. Base model `discogs-effnet-bs64-1.pb`

По графу:

```text
INPUT serving_default_melspectrogram shape [64,128,96]
INPUT saver_filename shape {}
```

Рабочий input только:

```text
serving_default_melspectrogram:0
```

`saver_filename` — служебный placeholder от TensorFlow Saver, его не указываем.

Outputs:

```text
PartitionedCall
```

Для base-модели лучше конвертировать оба выхода, потому в Essentia Discogs EffNet часто есть несколько outputs, а вам нужен тот, который даёт patch embeddings `[64,1280]`.

Команда:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/discogs-effnet-bs64-1.pb \
  --output ray-player1/assets/models/essentia/discogs-effnet-bs64-1.onnx \
  --inputs 'serving_default_melspectrogram:0[64,128,96]' \
  --outputs 'PartitionedCall:0,PartitionedCall:1' \
  --opset 13
```

После конвертации обязательно проверить, какой output имеет shape:

```text
[64,1280]
```

Именно его использовать как embeddings.

---

# 2. Проверка ONNX base model

```bash
uv run --with onnx python - <<'PY'
import onnx

path = "ray-player1/assets/models/essentia/discogs-effnet-bs64-1.onnx"

model = onnx.load(path)
onnx.checker.check_model(model)

print("OK:", path)

print("\nINPUTS:")
for i in model.graph.input:
    print(i.name, i.type)

print("\nOUTPUTS:")
for o in model.graph.output:
    print(o.name, o.type)
PY
```

В runtime ищите output, у которого итоговая форма:

```text
[64,1280]
```

Если после конвертации `PartitionedCall:1` отсутствует или tf2onnx ругается, тогда пробуйте только:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/discogs-effnet-bs64-1.pb \
  --output ray-player1/assets/models/essentia/discogs-effnet-bs64-1.onnx \
  --inputs 'serving_default_melspectrogram:0[64,128,96]' \
  --outputs 'PartitionedCall:0' \
  --opset 13
```

Но для вашей архитектуры я бы сначала оставил оба:

```text
PartitionedCall:0,PartitionedCall:1
```

---

# 3. Genre head `genre_discogs400-discogs-effnet-1.pb`

По графу:

```text
INPUT serving_default_model_Placeholder shape [-1,1280]
INPUT saver_filename shape {}
OUTPUT PartitionedCall
```

Правильная команда:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/genre_discogs400-discogs-effnet-1.pb \
  --output ray-player1/assets/models/essentia/genre_discogs400-discogs-effnet-1.onnx \
  --inputs 'serving_default_model_Placeholder:0[-1,1280]' \
  --outputs 'PartitionedCall:0' \
  --opset 13
```

Это верно для head-моделей, которые принимают raw patch embeddings:

```text
[validPatches,1280]
```

и выдают:

```text
[validPatches,numClasses]
```

Для genre Discogs400:

```text
[validPatches,400]
```

---

# 4. Проверка ONNX genre head

```bash
uv run --with onnx python - <<'PY'
import onnx

path = "ray-player1/assets/models/essentia/genre_discogs400-discogs-effnet-1.onnx"

model = onnx.load(path)
onnx.checker.check_model(model)

print("OK:", path)

print("\nINPUTS:")
for i in model.graph.input:
    print(i.name, i.type)

print("\nOUTPUTS:")
for o in model.graph.output:
    print(o.name, o.type)
PY
```

Ожидаемо:

```text
input:  serving_default_model_Placeholder:0 [-1,1280]
output: PartitionedCall:0 [-1,400]
```

---

# 5. Команды для всех EffNet heads

Если у всех head-моделей тот же input:

```text
serving_default_model_Placeholder:0[-1,1280]
```

и output:

```text
PartitionedCall:0
```

то их можно конвертировать одинаково.

Пример для mood happy:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/mood_happy-discogs-effnet-1.pb \
  --output ray-player1/assets/models/essentia/mood_happy-discogs-effnet-1.onnx \
  --inputs 'serving_default_model_Placeholder:0[-1,1280]' \
  --outputs 'PartitionedCall:0' \
  --opset 13
```

Пример для danceability:

```bash
uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
  --graphdef ray-player1/assets/models/essentia/danceability-discogs-effnet-1.pb \
  --output ray-player1/assets/models/essentia/danceability-discogs-effnet-1.onnx \
  --inputs 'serving_default_model_Placeholder:0[-1,1280]' \
  --outputs 'PartitionedCall:0' \
  --opset 13
```

---

# 6. Batch convert script для всех heads

Можно так:

```bash
MODELDIR="ray-player1/assets/models/essentia"

for f in \
  genre_discogs400-discogs-effnet-1 \
  danceability-discogs-effnet-1 \
  mood_happy-discogs-effnet-1 \
  mood_sad-discogs-effnet-1 \
  mood_relaxed-discogs-effnet-1 \
  mood_party-discogs-effnet-1 \
  mood_aggressive-discogs-effnet-1 \
  mood_acoustic-discogs-effnet-1 \
  mood_electronic-discogs-effnet-1 \
  voice_instrumental-discogs-effnet-1
do
  echo "Converting $f.pb -> $f.onnx"

  uv run --with tf2onnx --with tensorflow python -m tf2onnx.convert \
    --graphdef "$MODELDIR/$f.pb" \
    --output "$MODELDIR/$f.onnx" \
    --inputs 'serving_default_model_Placeholder:0[-1,1280]' \
    --outputs 'PartitionedCall:0' \
    --opset 13
done
```

---

# 7. Важное runtime-правило

После конвертации pipeline должен быть таким:

```text
audio
→ mel [64,128,96]
→ discogs-effnet-bs64-1.onnx
→ embeddings [64,1280]
→ valid embeddings [validPatches,1280]
→ genre/mood/danceability heads
→ average predictions over patches
```

Не так:

```text
average embedding
→ L2 normalize
→ head model
```

А именно так:

```text
raw patch embeddings [validPatches,1280]
→ head
→ average head predictions
```

Для recommendation embedding уже отдельно:

```text
average base embeddings
→ L2 normalize
→ save/use for similarity
```

---

# 8. Что добавить в код валидации

На старте приложения проверить:

```go
base input == [64,128,96]
base output contains [64,1280]

head input second dim == 1280
genre output classes == 400
binary mood heads output classes == 2 или 1, в зависимости от модели
```

Логи:

```text
INFO  | EssentiaEngine | ✅ Base model: discogs-effnet-bs64-1.onnx
DEBUG | EssentiaEngine | base output[0] shape=[64,1280]
INFO  | EssentiaEngine | ✅ Head genre_discogs400 input=[-1,1280] output=[-1,400]
```

---

# 9. Итог

Для base:

```bash
--inputs 'serving_default_melspectrogram:0[64,128,96]'
--outputs 'PartitionedCall:0,PartitionedCall:1'
```

Для всех Discogs EffNet heads:

```bash
--inputs 'serving_default_model_Placeholder:0[-1,1280]'
--outputs 'PartitionedCall:0'
```

`saver_filename` игнорируем. Это не runtime input модели, а служебный placeholder TensorFlow Saver.
