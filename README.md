# 🃏 Anki Server (Go Edition)

Высокопроизводительный локальный сервер для интеграции Safari с Anki. Написан на Go, поставляется как Docker-образ.

## 🚀 Быстрый старт

### Через Docker (рекомендуется)

```bash
# 1. Скопируй конфиг
cp .env.example .env
# Вставь свой DeepL ключ в DEEPL_KEY=

# 2. Запусти
docker compose up -d
```

Сервер доступен на `http://localhost:5005`.

### Coolify

1. В Coolify создай новый сервис: **Source → Git Repository**
2. Вставь ссылку на репо: `git@github.com:punjet/ankiServer.git`
3. Build Pack: **Docker Compose**
4. Добавь переменную окружения `DEEPL_KEY` — и жми Deploy!

> Образ берётся из `ghcr.io/punjet/ankiserver:latest` — пересобирается автоматически при каждом пуше в `main`.

---

## 📡 API Endpoints

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/add` | Добавить слово в буфер |
| `POST` | `/sync` | Выгрузить буфер в Anki |
| `POST` | `/translate` | Перевести слово через DeepL |
| `POST` | `/check` | Проверить, есть ли слово в Anki |
| `POST` | `/grammar` | 🧠 Анализ грамматики → Anki карточки |
| `GET` | `/config` | Текущая конфигурация |
| `POST` | `/config` | Обновить DeepL / OpenAI ключ |
| `GET` | `/health` | Health-check (для Docker) |

### POST `/grammar`

Принимает английский текст, находит грамматические ошибки через GPT-4o-mini и автоматически создаёт Anki-карточки.

**Request:**
```json
{
  "text": "I goed to store yesterday and buyed milk",
  "source_url": "https://example.com"
}
```

**Response:**
```json
{
  "errors_found": 2,
  "text_corrected": "I went to the store yesterday and bought milk",
  "cards_added": 2,
  "cards": [
    {
      "type": "correction",
      "error_fragment": "goed",
      "front": "Complete correctly: \"I ___ to the store yesterday\"",
      "back": "✅ went\n\n📌 Rule: \"go\" is irregular: go → went\n\n💡 More examples:\n• buy → bought\n• think → thought",
      "rule_tag": "irregular_verbs",
      "difficulty": "medium"
    }
  ]
}
```

Карточки сохраняются в буфер и попадают в Anki при следующем `/sync`.

---

---

## ⚙️ Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `PORT` | `5005` | Порт HTTP сервера |
| `ANKI_URL` | `http://host.docker.internal:8765` | URL AnkiConnect |
| `DEEPL_KEY` | — | API ключ DeepL (обязательный) |
| `BUFFER_FILE` | `/data/anki_buffer.json` | Путь к файлу буфера |
| `LOG_LEVEL` | `info` | Уровень логов |

---

## 🏗️ Структура проекта

```
ankiServer/
├── server/                     # Go сервер
│   ├── cmd/main.go             # Точка входа, роутинг
│   ├── internal/
│   │   ├── anki/client.go      # AnkiConnect клиент
│   │   ├── buffer/buffer.go    # Thread-safe JSON буфер
│   │   ├── config/config.go    # Конфигурация из env
│   │   ├── deepl/client.go     # DeepL API клиент
│   │   ├── handlers/           # HTTP хендлеры
│   │   └── middleware/cors.go  # CORS
│   └── Dockerfile              # Multi-stage build
├── userscript.js               # Tampermonkey скрипт для Safari
├── docker-compose.yml          # Готов для Coolify
├── .env.example                # Шаблон конфигурации
└── .github/workflows/          # CI/CD → GHCR
```

---

## 📂 Поля модели Anki

Модель `WordsFromSafari` должна содержать поля:
`Word`, `WordTranslation`, `Context`, `ContextTranslation`, `Audio`, `Spelling`, `SpellingTranscript`, `SourceURL`, `DateAdded`, `SeenCount`

Сервер проверяет наличие полей при старте и добавляет недостающие автоматически.

---

## 🛠️ Разработка

```bash
cd server
go mod tidy
go run ./cmd
```

```bash
# Собрать Docker образ локально
docker build -t anki-server ./server
docker run -p 5005:5005 -e DEEPL_KEY=your-key anki-server
```

---

## 📝 Лицензия
MIT
