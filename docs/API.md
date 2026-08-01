# Anki Server — Полная документация API

> **Версия:** 2.0 · **Base URL:** `http://localhost:5005`  
> **Формат:** все запросы и ответы — `application/json`

---

## Содержание

1. [Аутентификация](#1-аутентификация)
2. [Коды ошибок](#2-коды-ошибок)
3. [Rate Limiting](#3-rate-limiting)
4. [Эндпоинты](#4-эндпоинты)
   - [POST /add](#post-add)
   - [POST /sync](#post-sync)
   - [POST /translate](#post-translate)
   - [POST /check](#post-check)
   - [POST /grammar 🧠](#post-grammar-)
   - [GET /config](#get-config)
   - [POST /config](#post-config)
   - [GET /health](#get-health)
5. [Переменные окружения](#5-переменные-окружения)
6. [Безопасность](#6-безопасность)
7. [Деплой на Coolify](#7-деплой-на-coolify)
8. [Примеры интеграции](#8-примеры-интеграции)

---

## 1. Аутентификация

Если переменная `API_KEY` задана, **все запросы** (кроме `/health`) должны включать ключ.

### Способ 1 — заголовок `X-API-Key` (рекомендуется)

```http
POST /grammar HTTP/1.1
X-API-Key: your-secret-key
Content-Type: application/json
```

### Способ 2 — `Authorization: Bearer`

```http
POST /grammar HTTP/1.1
Authorization: Bearer your-secret-key
Content-Type: application/json
```

### Если ключ не передан или неверный

```json
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer realm="anki-server"

{
  "error": "unauthorized",
  "detail": "missing API key — provide X-API-Key header or Authorization: Bearer <key>"
}
```

> [!NOTE]
> Если `API_KEY` **не задан** в env — аутентификация отключена (удобно для локального использования). В продакшене всегда задавайте ключ.

---

## 2. Коды ошибок

| HTTP статус | Когда возникает |
|---|---|
| `200 OK` | Запрос успешен |
| `400 Bad Request` | Неверный JSON / отсутствует обязательное поле |
| `401 Unauthorized` | Неверный или отсутствующий API ключ |
| `422 Unprocessable Entity` | Данные валидны, но нарушают бизнес-правило (текст слишком длинный) |
| `429 Too Many Requests` | Превышен rate limit |
| `500 Internal Server Error` | Ошибка сервера / недоступна зависимость |

### Формат ошибки

```json
{
  "error": "machine_readable_code",
  "detail": "Human-readable explanation in English"
}
```

---

## 3. Rate Limiting

| Эндпоинт | Лимит | Окно |
|---|---|---|
| `/grammar` | 10 req/min | скользящее 60 сек |
| Все остальные | 60 req/min | скользящее 60 сек |
| `/health` | без лимита | — |

При превышении лимита:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60

{
  "error": "rate_limit_exceeded",
  "detail": "too many requests — slow down and retry after 60s"
}
```

Лимиты настраиваются через `RATE_LIMIT_DEFAULT` и `RATE_LIMIT_GRAMMAR`.

---

## 4. Эндпоинты

---

### POST /add

Добавляет слово/фразу в локальный JSON-буфер. Возвращает ответ немедленно — запись в Anki происходит при `/sync`.

#### Request Body

```json
{
  "action": "addNote",
  "sourceUrl": "https://example.com/article",
  "params": {
    "note": {
      "deckName": "WordsFromSafari",
      "modelName": "WordsFromSafari",
      "fields": {
        "Word": "ephemeral",
        "WordTranslation": "эфемерный",
        "Context": "These ephemeral moments define our lives.",
        "ContextTranslation": "Эти мимолётные моменты определяют нашу жизнь."
      },
      "tags": ["example", "reddit"]
    }
  }
}
```

| Поле | Тип | Обязателен | Описание |
|---|---|---|---|
| `action` | string | ✅ | Всегда `"addNote"` |
| `sourceUrl` | string | — | URL страницы, откуда добавлено слово |
| `params.note.deckName` | string | ✅ | Колода Anki |
| `params.note.modelName` | string | ✅ | Тип заметки Anki |
| `params.note.fields.Word` | string | ✅ | Само слово |
| `params.note.fields.WordTranslation` | string | — | Перевод |
| `params.note.fields.Context` | string | — | Предложение-контекст |
| `params.note.fields.ContextTranslation` | string | — | Перевод контекста |

#### Поля, заполняемые сервером автоматически

| Поле | Что вставляет сервер |
|---|---|
| `DateAdded` | Текущая дата `YYYY-MM-DD` |
| `SourceURL` | Из `sourceUrl` запроса |
| `SeenCount` | `"1"` |
| `Audio`, `Spelling`, `SpellingTranscript` | Пустые строки (заглушки) |
| tags | Добавляет домен из URL (напр. `reddit`) |

#### Response

```json
{ "status": "saved" }
```

#### curl пример

```bash
curl -X POST http://localhost:5005/add \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{
    "action": "addNote",
    "sourceUrl": "https://reddit.com/r/programming",
    "params": {
      "note": {
        "deckName": "WordsFromSafari",
        "modelName": "WordsFromSafari",
        "fields": {
          "Word": "ephemeral",
          "WordTranslation": "эфемерный",
          "Context": "These ephemeral moments define our lives.",
          "ContextTranslation": "Эти мимолётные моменты определяют нашу жизнь."
        }
      }
    }
  }'
```

---

### POST /sync

Отправляет все накопленные в буфере заметки в Anki через AnkiConnect.  
Anki **должна быть запущена** с установленным дополнением AnkiConnect.

Дубликаты не вызывают ошибку — вместо этого сервер инкрементирует поле `SeenCount`.

#### Request Body

Пустой объект `{}` или пустое тело.

#### Response

```json
{
  "status": "done",
  "added": 5,
  "duplicates": 1,
  "remaining": 0
}
```

| Поле | Описание |
|---|---|
| `added` | Успешно добавлено в Anki |
| `duplicates` | Дубликатов (SeenCount увеличен) |
| `remaining` | Осталось в буфере (не удалось отправить) |

Если буфер пуст:

```json
{ "status": "empty" }
```

#### curl пример

```bash
curl -X POST http://localhost:5005/sync \
  -H "X-API-Key: your-secret-key"
```

---

### POST /translate

Переводит слово или фразу с английского на русский через DeepL API.

Если передан `context`, используется **context-aware** перевод: слово оборачивается в XML-тег внутри предложения, DeepL переводит всё предложение и возвращает перевод конкретного слова в нужной форме.

#### Request Body

```json
{
  "text": "substantial",
  "context": "This is a substantial improvement over the previous version."
}
```

| Поле | Тип | Обязателен | Описание |
|---|---|---|---|
| `text` | string | ✅ | Слово или фраза для перевода |
| `context` | string | — | Предложение, в котором встретилось слово |

#### Response (с контекстом)

```json
{
  "translation": "существенное",
  "context_translation": "Это существенное улучшение по сравнению с предыдущей версией."
}
```

#### Response (без контекста)

```json
{
  "translation": "существенный"
}
```

#### Ошибки

| Статус | `error` | Причина |
|---|---|---|
| `400` | `no text` | Поле `text` пусто |
| `400` | `DeepL key not set` | `DEEPL_KEY` не задан |
| `401` | `invalid DeepL key` | Ключ неверный (403 от DeepL) |
| `500` | `DeepL error: ...` | Проблемы на стороне DeepL |

#### curl пример

```bash
curl -X POST http://localhost:5005/translate \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{"text": "substantial", "context": "This is a substantial improvement."}'
```

---

### POST /check

Проверяет, есть ли слово уже в Anki (колода `WordsFromSafari`).  
Используется для показа кнопки «already in anki · seen N×» в userscript.

#### Request Body

```json
{ "word": "ephemeral" }
```

#### Response

```json
{
  "exists": true,
  "seen_count": 3
}
```

Если слово не найдено:

```json
{
  "exists": false,
  "seen_count": 0
}
```

> [!NOTE]
> При недоступном Anki возвращает `{ "exists": false, "seen_count": 0 }` без ошибки — это намеренно, чтобы не блокировать UI.

#### curl пример

```bash
curl -X POST http://localhost:5005/check \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{"word": "ephemeral"}'
```

---

### POST /grammar 🧠

**Главный эндпоинт.** Анализирует английский текст через GPT-4o-mini, находит грамматические/лексические ошибки и генерирует структурированные Anki-карточки.

Карточки автоматически добавляются в буфер и попадут в Anki при следующем `/sync`.

#### Request Body

```json
{
  "text": "I goed to store yesterday and buyed some milk. She don't like it.",
  "source_url": "https://example.com"
}
```

| Поле | Тип | Обязателен | Описание |
|---|---|---|---|
| `text` | string | ✅ | Английский текст для анализа (3–4000 символов) |
| `source_url` | string | — | Источник текста (сохраняется в карточке) |

**Ограничения:**
- Минимум: 3 слова
- Максимум: 4000 символов (настраивается через `MAX_TEXT_LENGTH`)
- Max body: 64 KB

#### Response

```json
{
  "errors_found": 3,
  "text_corrected": "I went to the store yesterday and bought some milk. She doesn't like it.",
  "cards_added": 3,
  "cards": [
    {
      "type": "correction",
      "error_fragment": "goed",
      "front": "Complete correctly: \"I ___ to the store yesterday\"",
      "back": "✅ went\n\n📌 Rule: \"go\" is an irregular verb.\nPast Simple: go → went (NOT \"goed\")\n\n💡 More examples:\n• buy → bought\n• think → thought\n• run → ran",
      "rule_tag": "irregular_verbs",
      "difficulty": "medium"
    },
    {
      "type": "correction",
      "error_fragment": "buyed",
      "front": "What is the Past Simple of \"buy\"?",
      "back": "✅ bought\n\n📌 \"buy\" is irregular: buy → bought\n\n💡 Related forms:\n• buy → bought → bought (Past Participle)\n• bring → brought | think → thought",
      "rule_tag": "irregular_verbs",
      "difficulty": "easy"
    },
    {
      "type": "rule",
      "error_fragment": "She don't like",
      "front": "Fix the verb agreement: \"She ___ like spicy food.\"",
      "back": "✅ She doesn't like spicy food.\n\n📌 Rule: With he/she/it in Present Simple,\nuse does + base verb for negation (NOT do).\n\n💡 Examples:\n• He doesn't want to leave.\n• She doesn't know the answer.",
      "rule_tag": "subject_verb_agreement",
      "difficulty": "easy"
    }
  ]
}
```

#### Типы карточек (`type`)

| Тип | Описание | Пример ошибки |
|---|---|---|
| `correction` | Прямое исправление формы слова | `goed`, `buyed`, `runned` |
| `rule` | Грамматическое правило, нарушенное в тексте | `she don't`, `I am go`, `he have` |
| `word_choice` | Выбор слова, коллокация, предлог | `make homework`, `say him`, `go to home` |

#### Поля карточки

| Поле | Описание |
|---|---|
| `type` | Тип карточки (см. выше) |
| `error_fragment` | Фрагмент оригинального текста с ошибкой |
| `front` | Лицевая сторона карточки (вопрос/задание) |
| `back` | Оборотная сторона (ответ + объяснение + примеры) |
| `rule_tag` | Тег правила (`irregular_verbs`, `articles`, `prepositions`, ...) |
| `difficulty` | `easy` / `medium` / `hard` |

#### Если текст без ошибок

```json
{
  "errors_found": 0,
  "text_corrected": "Your text here",
  "cards_added": 0,
  "cards": []
}
```

#### Ошибки

| Статус | `error` | Причина |
|---|---|---|
| `400` | `text_too_short` | Меньше 3 слов |
| `400` | `OpenAI key not configured` | `OPENAI_API_KEY` не задан |
| `401` | `invalid OpenAI API key` | Ключ неверный |
| `422` | `text_too_long` | Превышен `MAX_TEXT_LENGTH` |
| `429` | `rate_limit_exceeded` | Слишком много запросов |

#### curl пример

```bash
curl -X POST http://localhost:5005/grammar \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{
    "text": "I goed to store yesterday and buyed milk. She don'\''t like spicy food.",
    "source_url": "https://example.com"
  }'
```

---

### GET /config

Возвращает текущее состояние сервера — какие ключи настроены, статистика использования DeepL.

#### Response

```json
{
  "has_deepl_key": true,
  "has_openai_key": true,
  "deepl_chars_this_month": 12450,
  "deepl_chars_month": "2026-08"
}
```

| Поле | Описание |
|---|---|
| `has_deepl_key` | Задан ли DeepL ключ |
| `has_openai_key` | Задан ли OpenAI ключ |
| `deepl_chars_this_month` | Символов использовано в текущем месяце |
| `deepl_chars_month` | Текущий месяц в формате `YYYY-MM` |

> [!NOTE]
> Ключи **никогда** не возвращаются в ответе — только факт их наличия.

---

### POST /config

Обновляет ключи на лету **без перезапуска** сервера.

#### Request Body

```json
{
  "deepl_key": "your-deepl-api-key:fx",
  "openai_key": "sk-proj-..."
}
```

Оба поля опциональны — можно обновить только один ключ.

#### Response

```json
{ "status": "ok" }
```

#### curl пример

```bash
# Обновить только DeepL ключ
curl -X POST http://localhost:5005/config \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{"deepl_key": "your-new-key:fx"}'

# Обновить только OpenAI ключ
curl -X POST http://localhost:5005/config \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-secret-key" \
  -d '{"openai_key": "sk-proj-..."}'
```

---

### GET /health

Health check для Docker / Coolify. **Не требует аутентификации.**

#### Response

```json
{ "status": "ok", "version": "2.0" }
```

---

## 5. Переменные окружения

### Основные

| Переменная | По умолчанию | Обязателен | Описание |
|---|---|---|---|
| `PORT` | `5005` | — | HTTP порт |
| `ANKI_URL` | `http://host.docker.internal:8765` | — | URL AnkiConnect |
| `BUFFER_FILE` | `/data/anki_buffer.json` | — | Путь к файлу буфера |
| `LOG_LEVEL` | `info` | — | `debug` / `info` / `warn` / `error` |

### Внешние API

| Переменная | По умолчанию | Обязателен | Описание |
|---|---|---|---|
| `DEEPL_KEY` | — | Для `/translate` | DeepL API ключ. Free ключи оканчиваются на `:fx` |
| `OPENAI_API_KEY` | — | Для `/grammar` | OpenAI API ключ (`sk-proj-...`) |
| `OPENAI_MODEL` | `gpt-4o-mini` | — | Модель. Варианты: `gpt-4o-mini`, `gpt-4o`, `gpt-4-turbo` |
| `MAX_CARDS_PER_REQUEST` | `5` | — | Макс. карточек за один `/grammar` запрос |

### Anki модели

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRAMMAR_DECK_NAME` | `GrammarErrors` | Колода для грамматических карточек |
| `GRAMMAR_MODEL_NAME` | `GrammarErrors` | Тип заметки для грамматических карточек |

### Безопасность

| Переменная | По умолчанию | Описание |
|---|---|---|
| `API_KEY` | — | Секретный ключ сервера. Если не задан — auth отключена |
| `RATE_LIMIT_DEFAULT` | `60` | Req/min для стандартных эндпоинтов |
| `RATE_LIMIT_GRAMMAR` | `10` | Req/min для `/grammar` (AI calls expensive) |
| `MAX_BODY_BYTES` | `65536` | Макс. размер тела запроса (64 KB) |
| `MAX_TEXT_LENGTH` | `4000` | Макс. длина текста в `/grammar` (в символах) |
| `TRUSTED_PROXIES` | `true` | Доверять X-Forwarded-For (нужно при работе за nginx/Coolify) |

---

## 6. Безопасность

### Что реализовано

#### Аутентификация
- Timing-safe сравнение (`crypto/subtle.ConstantTimeCompare`) — защита от timing attacks
- Принимается как `X-API-Key`, так и `Authorization: Bearer`
- `/health` всегда публичный (нужен для health checks)

#### Rate Limiting
- Скользящее окно (sliding window) per-IP
- Разные лимиты для дорогих (AI) и дешёвых эндпоинтов
- Автоматическая очистка памяти от неактивных IP

#### HTTP заголовки
```http
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Referrer-Policy: no-referrer
Content-Security-Policy: default-src 'none'
Permissions-Policy: camera=(), microphone=(), geolocation=()
```
Заголовки `Server` и `X-Powered-By` удаляются.

#### Размер запроса
- Max body: 64 KB (настраивается)
- Max text: 4000 символов в `/grammar`
- Unicode-aware (считает rune, не байты)

#### Тайм-ауты
```go
ReadHeaderTimeout: 5s   // защита от slow-loris
ReadTimeout:       15s  // чтение тела запроса
WriteTimeout:      90s  // ответ (с запасом для AI)
IdleTimeout:       60s  // keep-alive соединения
```

#### Безопасность данных
- API ключи **никогда не логируются** и не возвращаются в ответах
- Атомарная запись буфера (write to `.tmp` → rename) — защита от corruption
- `.env` и `*.key` файлы в `.gitignore`

### Рекомендации для продакшена

```bash
# Минимальный безопасный запуск
API_KEY=$(openssl rand -hex 32)   # генерация сильного ключа

# Запускать только через HTTPS!
# Используйте Coolify / nginx с TLS-терминацией

# Не открывайте порт 5005 напрямую в интернет —
# пусть reverse proxy (Coolify/nginx) принимает HTTPS
# и проксирует на localhost:5005
```

> [!CAUTION]
> Если сервер доступен из интернета без HTTPS — API ключ передаётся в открытом виде. Всегда используйте TLS-терминацию (Coolify делает это автоматически).

---

## 7. Деплой на Coolify

### Шаг 1 — Создать сервис

1. **New Resource → Git Repository**
2. **Repository:** `git@github.com:punjet/ankiServer.git`
3. **Branch:** `main`
4. **Build Pack:** `Docker Compose`

### Шаг 2 — Переменные окружения в Coolify

```env
API_KEY=<openssl rand -hex 32>
DEEPL_KEY=your-deepl-key:fx
OPENAI_API_KEY=sk-proj-...
ANKI_URL=http://host.docker.internal:8765
```

### Шаг 3 — Deploy

Coolify автоматически:
- Найдёт `docker-compose.yml` в корне репо
- Поднимет контейнер с образом из GHCR
- Настроит HTTPS домен
- Настроит health check на `/health`

### Автообновление образа

GitHub Actions собирает новый образ при каждом `push` в `main` и публикует его в `ghcr.io/punjet/ankiserver:latest`.

В Coolify включите **"Auto Deploy on image update"** — сервис будет перезапускаться автоматически.

---

## 8. Примеры интеграции

### JavaScript (userscript / fetch)

```javascript
const SERVER = 'http://127.0.0.1:5005';
const API_KEY = 'your-secret-key'; // или хранить в GM_getValue

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY
};

// Перевести слово
async function translate(word, context) {
  const res = await fetch(`${SERVER}/translate`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ text: word, context })
  });
  return res.json(); // { translation, context_translation? }
}

// Анализ грамматики
async function analyzeGrammar(text) {
  const res = await fetch(`${SERVER}/grammar`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ text, source_url: location.href })
  });
  return res.json(); // { errors_found, text_corrected, cards_added, cards[] }
}

// Синхронизировать буфер в Anki
async function syncToAnki() {
  const res = await fetch(`${SERVER}/sync`, { method: 'POST', headers });
  return res.json(); // { status, added, duplicates, remaining }
}
```

### Python

```python
import httpx

client = httpx.Client(
    base_url="http://localhost:5005",
    headers={"X-API-Key": "your-secret-key"},
    timeout=30.0
)

# Грамматика
result = client.post("/grammar", json={
    "text": "I goed to store yesterday and buyed milk.",
    "source_url": "https://example.com"
})
data = result.json()
print(f"Errors found: {data['errors_found']}")
print(f"Cards added: {data['cards_added']}")
for card in data['cards']:
    print(f"\n[{card['rule_tag']}] {card['front']}")

# Синк
sync = client.post("/sync")
print(sync.json())
```

### curl — полный workflow

```bash
BASE="http://localhost:5005"
KEY="your-secret-key"
AUTH="-H 'X-API-Key: $KEY'"

# 1. Проверить здоровье
curl $BASE/health

# 2. Добавить слово
curl -X POST $BASE/add $AUTH \
  -H "Content-Type: application/json" \
  -d '{"action":"addNote","params":{"note":{"deckName":"WordsFromSafari","modelName":"WordsFromSafari","fields":{"Word":"serendipity","WordTranslation":"удача"}}}}'

# 3. Анализ грамматики  
curl -X POST $BASE/grammar $AUTH \
  -H "Content-Type: application/json" \
  -d '{"text":"Yesterday I have went to the store and buyed some milk."}'

# 4. Синхронизировать всё в Anki
curl -X POST $BASE/sync $AUTH

# 5. Проверить статус
curl $BASE/config $AUTH
```

---

### Поля модели Anki — `WordsFromSafari`

| Поле | Описание |
|---|---|
| `Word` | Слово |
| `WordTranslation` | Перевод |
| `Context` | Предложение с контекстом |
| `ContextTranslation` | Перевод контекста |
| `Audio` | Аудио файл `[sound:...]` |
| `Spelling` | Транскрипция |
| `SpellingTranscript` | Субтитры к аудио |
| `SourceURL` | URL источника |
| `DateAdded` | Дата добавления |
| `SeenCount` | Счётчик встреч |

### Поля модели Anki — `GrammarErrors`

| Поле | Описание |
|---|---|
| `Front` | Лицевая сторона карточки |
| `Back` | Оборотная сторона с объяснением |
| `RuleTag` | Тег правила (`irregular_verbs`, `articles`, ...) |
| `OriginalText` | Оригинальный текст с ошибкой |
| `CorrectedText` | Исправленная версия |
| `Difficulty` | `easy` / `medium` / `hard` |
| `DateAdded` | Дата создания |
| `ErrorType` | `correction` / `rule` / `word_choice` |
