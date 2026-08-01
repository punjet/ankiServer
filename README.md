# 🃏 Anki Server (Go Edition)

Высокопроизводительный облачный сервер для интеграции Safari с Anki. Написан на Go, работает в связке с вашим VPS (Coolify) и локальным скриптом `ankiupd.py`.

## 🚀 Деплой через Coolify

1. В Coolify создай новый сервис: **Source → Git Repository**
2. Выбери репозиторий: `ankiServer`
3. **Build Pack**: `Dockerfile`
4. **Base Directory**: `/`
5. **Dockerfile Location**: `/server/Dockerfile`

### Настройка постоянной памяти (Persistent Storage)
Чтобы при перезапусках сервера не пропадали логи и несохраненные карточки:
1. Вкладка **Storages** -> **Add Storage**
2. **Volume Name**: `anki_data`
3. **Destination Path**: `/app/data`
4. **Save**

### Переменные окружения (Environment Variables)
Добавьте следующие переменные:
```ini
# Ключ от сервера (вписывается в userscript.js)
API_KEY=your-secret-key

# API-ключи для перевода и аудио
DEEPL_KEY=abc12345:fx
OPENAI_API_KEY=sk-...

# Сохранение логов и буфера
LOG_FILE=/app/data/server.log
BUFFER_FILE=/app/data/anki_buffer.json
```
После этого нажимайте **Deploy**!

---

## 💻 Настройка клиента (Mac)

1. **Tampermonkey**: Скопируйте содержимое `userscript.js` в расширение Tampermonkey. В коде укажите ваш `API_KEY`.
2. **Терминал**: В файле `ankiupd.py` укажите адрес вашего сервера `SERVER_URL` и `API_KEY`.
3. Для удобства добавьте алиас в `~/.zshrc`:
   ```bash
   alias ankiupd="python3 /Users/YOUR_NAME/anki_server/ankiupd.py"
   ```
4. Теперь, чтобы забрать карточки с сервера и отправить в Anki, просто введите команду `ankiupd` в терминале.

---

## 📡 API Endpoints

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/add` | Добавить слово в буфер (генерирует аудио через OpenAI) |
| `POST` | `/translate` | Перевести слово через DeepL |
| `POST` | `/check` | Проверить, есть ли слово в Anki (через userscript) |
| `POST` | `/grammar` | 🧠 Анализ грамматики → Anki карточки |
| `GET`  | `/buffer` | Забрать все карточки (для `ankiupd.py`) |
| `DELETE`| `/buffer` | Удалить загруженные карточки из буфера |
| `GET`  | `/health` | Health-check (для Docker/Coolify) |

---

## ⚙️ Основные переменные окружения

| Переменная | Описание |
|---|---|
| `PORT` | Порт HTTP сервера (по умолчанию `5005`) |
| `API_KEY` | Секретный ключ для защиты вашего сервера |
| `DEEPL_KEY` | API ключ DeepL (обязательный для `/translate`) |
| `OPENAI_API_KEY` | Ключ OpenAI (для озвучки и проверки грамматики) |
| `OPENAI_MODEL` | Модель для грамматики (по умолчанию `gpt-4o-mini`) |
| `BUFFER_FILE` | Путь к файлу буфера (`/app/data/anki_buffer.json`) |
| `LOG_FILE` | Путь к файлу логов (`/app/data/server.log`) |

---

## 📂 Поля модели Anki

Модель `WordsFromSafari` должна содержать поля:
`Word`, `WordTranslation`, `Context`, `ContextTranslation`, `Audio`, `Spelling`, `SpellingTranscript`, `SourceURL`, `DateAdded`, `SeenCount`

Скрипт загрузки автоматически добавляет карточки в колоду и прикрепляет к ним сгенерированный звук (`[sound:tts_word.mp3]`). Если слово уже существует, скрипт обновит счетчик `SeenCount` (+1).

---

## 🛠️ Разработка

```bash
cd server
go mod tidy
go run ./cmd
```

```bash
# Запуск тестов
go test ./...
```

---

## 📝 Лицензия
MIT
