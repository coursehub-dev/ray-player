# Разбор метаданных: Что именно нужно для запуска в Go?

Смотреть нужно строго в блок **`schema`** предоставленного вами JSON-файла. Именно там прописана «инструкция» для движка ONNX Runtime в Go.

Давайте расшифруем эту метадату для вашего кода:

Вход модели настроения (`inputs`)

```json
"name": "model/Placeholder",
"type": "float",
"shape": [1280]
```

Используйте код с осторожностью.

- **Что это значит для Go:** Код должен передать в ONNX-сессию одномерный массив (тензор) типа `float32` строго из **1280 чисел**.
- **Откуда их взять:** Их вернет базовая модель `discogs-effnet-bs64-1.onnx` (Feature Extractor).

Выход модели настроения (`outputs`)

```json
"name": "model/Softmax",
"type": "float",
"shape": [2]
```

Используйте код с осторожностью.

- **Что это значит для Go:** На выходе ONNX вернет вам массив ровно из **2 чисел** типа `float32`.
- Поле `"classes": ["happy", "non_happy"]` из начала вашего JSON говорит, как их читать:
    - `Массив[0]` — это вероятность того, что трек **веселый** (happy).
    - `Массив[1]` — вероятность того, что трек **грустный/нейтральный** (non_happy).

---

Как это будет выглядеть в коде на Go (Wails)

Чтобы запустить эту модель в Go через официальный `onnxruntime-go`, вам нужно написать структуру, которая связывает эти метаданные. Вот как выглядит чистый рабочий каркас для запуска модели настроения:

```go
package main

import (
	"fmt"
	"log"

	ort "://github.com"
)

func main() {
	// 1. Инициализируем библиотеку ONNX Runtime (нужно вызвать один раз при старте Wails)
	ort.SetSharedLibraryPath("libonnxruntime.dylib") // Для macOS. Для Windows: "onnxruntime.dll"
	err := ort.InitializeEnvironment()
	if err != nil {
		log.Fatalf("Ошибка инициализации ONNX: %v", err)
	}
	defer ort.DestroyEnvironment()

	// 2. Загружаем модель mood_happy в память
	modelPath := "ray-player1/assets/models/essentia/mood_happy-discogs-effnet-1.onnx"
	
	// В Go inputNames и outputNames должны четко соответствовать "schema" из метаданных!
	inputNames := []string{"model/Placeholder"}
	outputNames := []string{"model/Softmax"}

	session, err := ort.NewAdvancedSession(modelPath, inputNames, outputNames, nil)
	if err != nil {
		log.Fatalf("Не удалось создать сессию: %v", err)
	}
	defer session.Destroy()

	// 3. Создаем фейковый входной вектор для теста (в реальности это 1280 чисел из базовой модели)
	// Shape модели: [1280]
	inputShape := ort.NewShape(1280)
	inputData := make([]float32, 1280) 
	// Заполним случайными данными для теста
	for i := range inputData {
		inputData[i] = 0.5 
	}

	inputTensor, err := ort.NewTensor(inputShape, inputData)
	if err != nil {
		log.Fatalf("Ошибка создания тензора: %v", err)
	}
	defer inputTensor.Destroy()

	// 4. Подготавливаем выходной тензор (Shape: [2])
	outputShape := ort.NewShape(2)
	outputData := make([]float32, 2)
	outputTensor, err := ort.NewTensor(outputShape, outputData)
	if err != nil {
		log.Fatalf("Ошибка создания выходного тензора: %v", err)
	}
	defer outputTensor.Destroy()

	// 5. Запускаем нейросеть (Инференс)
	err = session.Run([]ort.ArbitraryTensor{inputTensor}, []ort.ArbitraryTensor{outputTensor})
	if err != nil {
		log.Fatalf("Ошибка инференса: %v", err)
	}

	// 6. Читаем результат согласно классам в метаданных
	happyProb := outputData[0]
	sadProb := outputData[1]

	fmt.Printf("Анализ завершен!\n")
	fmt.Printf("Вероятность веселого трека (happy): %.2f%%\n", happyProb*100)
	fmt.Printf("Вероятность грустного трека (non_happy): %.2f%%\n", sadProb*100)
}
```
