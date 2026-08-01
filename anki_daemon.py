import json, os, requests, re, subprocess, sys, logging, tempfile, threading
from datetime import date
from urllib.parse import urlparse
from flask import Flask, request, jsonify
from flask_cors import CORS

# --- НАСТРОЙКА ЛОГИРОВАНИЯ ---
LOG_FILE = "/tmp/anki_daemon.log"
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    handlers=[
        logging.FileHandler(LOG_FILE),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

app = Flask(__name__)
CORS(app)

# --- КОНФИГУРАЦИЯ ---
DB_DIR = os.path.expanduser("~/anki_server")
DB_FILE = os.path.join(DB_DIR, "anki_buffer.json")
CONFIG_FILE = os.path.join(DB_DIR, "config.json")
ANKI_URL = "http://127.0.0.1:8765"

DEEPL_CONFIG_FILE = os.path.join(DB_DIR, ".deepl_key")

def load_config():
    if os.path.exists(CONFIG_FILE):
        with open(CONFIG_FILE, "r") as f:
            return json.load(f)
    return {}

def save_config(cfg):
    with open(CONFIG_FILE, "w") as f:
        json.dump(cfg, f, ensure_ascii=False, indent=2)

def get_deepl_key():
    if os.path.exists(DEEPL_CONFIG_FILE):
        with open(DEEPL_CONFIG_FILE, "r") as f:
            return f.read().strip()
    return None

def set_deepl_key(key):
    with open(DEEPL_CONFIG_FILE, "w") as f:
        f.write(key.strip())
    logger.info("🔑 DeepL ключ сохранён")

# ПУТИ К ПРОГРАММАМ
YT_DLP_PATH = "/Library/Frameworks/Python.framework/Versions/3.13/bin/yt-dlp"
FFMPEG_PATH = "/opt/homebrew/bin/ffmpeg"

# ПУТЬ К МЕДИА
ANKI_MEDIA_DIR = os.path.expanduser("~/Library/Application Support/Anki2/1-й пользователь/collection.media")

REQUIRED_FIELDS = ["Word", "WordTranslation", "Context", "ContextTranslation",
                   "Audio", "Spelling", "SpellingTranscript", "SourceURL", "DateAdded", "SeenCount"]
MODEL_NAME = "WordsFromSafari"
DECK_NAME  = "WordsFromSafari"

# --- СЧЁТЧИК СИМВОЛОВ DeepL ---
_deepl_chars_file = os.path.join(DB_DIR, ".deepl_chars")

def get_deepl_chars():
    if os.path.exists(_deepl_chars_file):
        try:
            with open(_deepl_chars_file) as f:
                return json.load(f)
        except: pass
    return {"month": "", "count": 0}

def add_deepl_chars(n):
    from datetime import date as _date
    month = _date.today().strftime("%Y-%m")
    data = get_deepl_chars()
    if data.get("month") != month:
        data = {"month": month, "count": 0}
    data["count"] += n
    with open(_deepl_chars_file, "w") as f:
        json.dump(data, f)

def ensure_model_fields():
    """Проверяет, что все нужные поля есть в модели Anki. Добавляет недостающие."""
    try:
        res = requests.post(ANKI_URL, json={
            "action": "modelFieldNames",
            "version": 6,
            "params": {"modelName": MODEL_NAME}
        }, timeout=5)
        existing = res.json().get("result", [])
        if existing is None:
            logger.warning("⚠️ Модель WordsFromSafari не найдена в Anki — пропускаем проверку полей")
            return
        missing = [f for f in REQUIRED_FIELDS if f not in existing]
        for field in missing:
            r = requests.post(ANKI_URL, json={
                "action": "addNoteField",
                "version": 6,
                "params": {"modelName": MODEL_NAME, "fieldName": field, "index": None}
            }, timeout=5)
            resp = r.json()
            if resp.get("error"):
                logger.warning(f"⚠️ Поле '{field}' отсутствует в модели и не может быть добавлено автоматически. "
                               f"Добавьте вручную: Anki → Browse → Fields. Ошибка: {resp['error']}")
            else:
                logger.info(f"➕ Добавлено поле '{field}'")
        if not missing:
            logger.info(f"✅ Все поля модели на месте: {existing}")
    except Exception as e:
        logger.warning(f"⚠️ ensure_model_fields: {e} (Anki не запущен?)")

if not os.path.exists(DB_DIR):
    os.makedirs(DB_DIR)
_whisper_model = None

def get_whisper_model():
    global _whisper_model
    if _whisper_model is None:
        logger.info("🧠 Загружаем Whisper модель...")
        import whisper
        _whisper_model = whisper.load_model("base")
        logger.info("✅ Whisper готов")
    return _whisper_model

def unload_whisper_model():
    global _whisper_model
    if _whisper_model is not None:
        _whisper_model = None
        import gc
        gc.collect()
        logger.info("🗑️ Whisper выгружен из памяти")

def trim_audio_to_word(raw_path, word, output_path):
    """Транскрибирует аудио через Whisper и вырезает точно слово."""
    PAD_START = 0.15
    PAD_END   = 0.35
    try:
        import whisper
        model = get_whisper_model()
        result = model.transcribe(raw_path, word_timestamps=True, language='en')

        word_start = None
        word_end   = None
        word_lower = word.lower()
        all_words = [w for seg in result['segments'] for w in seg['words']]

        # Проход 1: точное совпадение
        for w in all_words:
            clean = re.sub(r"[^a-zA-Z]", '', w['word']).lower()
            if clean == word_lower:
                word_start = w['start']
                word_end   = w['end']
                break

        # Проход 2 (fallback): слово в транскрипции — форма искомого (showing→show, shows→show)
        if word_start is None:
            for w in all_words:
                clean = re.sub(r"[^a-zA-Z]", '', w['word']).lower()
                if clean and (clean.startswith(word_lower) or word_lower.startswith(clean)):
                    word_start = w['start']
                    word_end   = w['end']
                    logger.info(f"🔍 Whisper fallback: '{clean}' → '{word_lower}'")
                    break

        if word_start is None:
            logger.warning(f"⚠️ Whisper не нашёл слово '{word}' в транскрипции: {result['text']}")
            return False

        t_start  = max(0, word_start - PAD_START)
        duration = (word_end + PAD_END) - t_start
        logger.info(f"✂️  '{word}' найдено {word_start:.2f}s-{word_end:.2f}s → вырезаем {t_start:.2f}s, {duration:.2f}s")

        cmd = [
            FFMPEG_PATH, '-y',
            '-i', raw_path,
            '-ss', str(t_start),
            '-t',  str(duration),
            '-ac', '1', '-ar', '22050', '-b:a', '32k',
            output_path
        ]
        subprocess.run(cmd, check=True, capture_output=True)
        return True

    except Exception as e:
        logger.error(f"💥 Ошибка trim_audio_to_word: {e}")
        return False

def get_youglish_data(word):
    logger.info(f"🔎 Поиск YouGlish: {word}")
    url = f"https://youglish.com/pronounce/{word}/english/us"
    headers = {'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36'}
    
    try:
        res = requests.get(url, headers=headers, timeout=10)
        html = res.text
        match = re.search(r"params\.jsonData\s*=\s*'(.*?)';", html, re.DOTALL)
        if not match: return None, None
            
        data_str = match.group(1)
        vid   = re.search(r'vid\\?":\\?"([^"\\]+)', data_str)
        start = re.search(r'start\\?":\\?"(\d+)', data_str)
        text  = re.search(r'display\\?":\\?"(.*?)\\?"', data_str)

        if not vid: return None, None

        video_id   = vid.group(1).replace('\\', '')
        start_time = int(start.group(1)) if start else 0
        sentence   = text.group(1).replace('\\', '') if text else ""

        filename    = f"yg_{word}_{video_id}.mp3"
        output_path = os.path.join(ANKI_MEDIA_DIR, filename)

        if not os.path.exists(output_path):
            logger.info(f"🚀 Скачивание отрывка для '{word}'")

            # Скачиваем 7 сек во временный файл
            with tempfile.NamedTemporaryFile(suffix='.mp3', delete=False) as tmp:
                raw_path = tmp.name

            cmd = [
                YT_DLP_PATH, "--ffmpeg-location", FFMPEG_PATH,
                "--extract-audio", "--audio-format", "mp3",
                "--download-sections", f"*{start_time}-{start_time + 7}",
                "--postprocessor-args", "ffmpeg:-ac 1 -ar 22050 -b:a 32k",
                f"https://www.youtube.com/watch?v={video_id}",
                "-o", raw_path, "--quiet", "--no-warnings"
            ]
            subprocess.run(cmd, check=True)

            # Вырезаем только слово через Whisper
            trimmed = trim_audio_to_word(raw_path, word, output_path)
            unload_whisper_model()  # сразу освобождаем RAM

            # Если Whisper не нашёл слово — сохраняем полный отрывок как fallback
            if not trimmed:
                logger.warning(f"⚠️ Whisper не нашёл '{word}', сохраняем полный отрывок")
                import shutil
                shutil.copy(raw_path, output_path)

            os.unlink(raw_path)
            logger.info(f"🎉 Файл сохранен: {filename}")
        else:
            logger.info(f"ℹ️ Файл уже существует: {filename}")

        return filename, sentence
    except Exception as e:
        logger.error(f"💥 Ошибка YouGlish: {e}")
        return None, None

# --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---

def extract_domain_tag(url):
    """reddit.com/... → 'reddit'"""
    try:
        host = urlparse(url).hostname or ""
        # убираем www.
        host = re.sub(r'^www\.', '', host)
        # берём первую часть: youtube.com → youtube
        return host.split('.')[0]
    except:
        return ""

def get_card_seen_count(word):
    """Ищет карточку в Anki и возвращает текущий SeenCount."""
    try:
        res = requests.post(ANKI_URL, json={
            "action": "findNotes",
            "version": 6,
            "params": {"query": f'Word:"{word}" deck:WordsFromSafari'}
        }, timeout=5)
        note_ids = res.json().get("result", [])
        if not note_ids:
            return None, None

        note_id = note_ids[0]
        info_res = requests.post(ANKI_URL, json={
            "action": "notesInfo",
            "version": 6,
            "params": {"notes": [note_id]}
        }, timeout=5)
        fields = info_res.json()["result"][0]["fields"]
        seen = int(fields.get("SeenCount", {}).get("value", "1") or "1")
        return note_id, seen
    except Exception as e:
        logger.error(f"❌ get_card_seen_count: {e}")
        return None, None

def increment_seen_count(note_id, current_count):
    """Обновляет SeenCount на карточке."""
    try:
        requests.post(ANKI_URL, json={
            "action": "updateNoteFields",
            "version": 6,
            "params": {
                "note": {
                    "id": note_id,
                    "fields": {"SeenCount": str(current_count + 1)}
                }
            }
        }, timeout=5)
        logger.info(f"🔢 SeenCount обновлён: {current_count} → {current_count + 1}")
    except Exception as e:
        logger.error(f"❌ increment_seen_count: {e}")
    if os.path.exists(DB_FILE):
        with open(DB_FILE, "r", encoding="utf-8") as f:
            try: return json.load(f)
            except: return []
    return []

_buffer_lock = threading.Lock()

def load_buffer():
    if os.path.exists(DB_FILE):
        with open(DB_FILE, "r", encoding="utf-8") as f:
            try: return json.load(f)
            except: return []
    return []

def save_buffer(buffer):
    with open(DB_FILE, "w", encoding="utf-8") as f:
        json.dump(buffer, f, ensure_ascii=False, indent=4)

# --- МАРШРУТЫ ---

def enrich_audio_in_background(word, note_id_in_buffer):
    """Скачивает аудио и дописывает поля в буфер по индексу."""
    try:
        audio_file, yg_sentence = get_youglish_data(word)
        if not audio_file:
            logger.warning(f"⚠️ Аудио не найдено для '{word}', карточка останется без аудио")
            return
        with _buffer_lock:
            buf = load_buffer()
            for note in buf:
                if note.get('_buf_id') == note_id_in_buffer:
                    note['params']['note']['fields']['Audio']             = f"[sound:{audio_file}]"
                    note['params']['note']['fields']['Spelling']          = yg_sentence
                    note['params']['note']['fields']['SpellingTranscript'] = yg_sentence
                    save_buffer(buf)
                    logger.info(f"🎵 Аудио дописано в буфер для '{word}'")
                    break
    except Exception as e:
        logger.error(f"💥 enrich_audio_in_background: {e}")

@app.route('/add', methods=['POST'])
def add():
    data = request.json
    try:
        fields = data['params']['note']['fields']
        word = fields.get('Word', 'Unknown')
    except:
        return jsonify({"status": "error"}), 400

    source_url = data.get('sourceUrl', '')
    domain_tag = extract_domain_tag(source_url)
    today = date.today().isoformat()

    fields['SourceURL']  = source_url
    fields['DateAdded']  = today
    fields['SeenCount']  = "1"
    fields['Audio']             = ""
    fields['Spelling']          = ""
    fields['SpellingTranscript'] = ""

    tags = data['params']['note'].get('tags', [])
    if domain_tag and domain_tag not in tags:
        tags.append(domain_tag)
    data['params']['note']['tags'] = tags

    # Уникальный ID для поиска записи в буфере из фонового треда
    import time
    buf_id = f"{word}_{int(time.time()*1000)}"
    data['_buf_id'] = buf_id

    with _buffer_lock:
        buffer = load_buffer()
        buffer.append(data)
        save_buffer(buffer)

    logger.info(f"📥 ДОБАВЛЕНИЕ: {word} | {domain_tag} | {source_url[:60]}")
    logger.info(f"💾 Сохранено в буфер. Очередь: {len(buffer)}. Аудио в фоне...")

    # Запускаем YouGlish + Whisper в фоне — не блокируем ответ
    t = threading.Thread(target=enrich_audio_in_background, args=(word, buf_id), daemon=True)
    t.start()

    return jsonify({"status": "saved"})

@app.route('/sync', methods=['POST'])
def sync():
    with _buffer_lock:
        buffer = load_buffer()
    if not buffer: return jsonify({"status": "empty"})
    
    logger.info(f"🔄 Синхронизация {len(buffer)} заметок...")
    remaining = []
    success = 0
    
    for note in buffer:
        word = note['params']['note']['fields'].get('Word', 'Unknown')
        try:
            res_raw = requests.post(ANKI_URL, json=note, timeout=5)
            res = res_raw.json()

            # ГИБКАЯ ПРОВЕРКА ОТВЕТА
            is_ok = False
            
            if isinstance(res, dict):
                if res.get("error") is None:
                    is_ok = True
                elif "duplicate" in str(res.get("error")).lower():
                    logger.warning(f"⚠️ Дубликат: {word} — обновляем SeenCount")
                    note_id, seen = get_card_seen_count(word)
                    if note_id:
                        increment_seen_count(note_id, seen)
                    is_ok = True
                else:
                    logger.error(f"❌ Ошибка Anki для {word}: {res['error']}")
            elif isinstance(res, (int, float)):
                # Если AnkiConnect вернул просто ID (число)
                is_ok = True

            if is_ok:
                logger.info(f"✅ Успешно: {word}")
                success += 1
            else:
                remaining.append(note)

        except Exception as e:
            logger.error(f"❗ Ошибка связи для {word}: {e}")
            remaining.append(note)
            
    with _buffer_lock:
        save_buffer(remaining)
    logger.info(f"🏁 Итог: +{success}, в буфере: {len(remaining)}")
    return jsonify({"status": "done", "added": success, "remaining": len(remaining)})

@app.route('/translate', methods=['POST'])
def translate():
    data = request.json
    text = data.get('text', '')
    context = data.get('context', None)
    
    if not text:
        return jsonify({"error": "no text"}), 400
    
    key = get_deepl_key()
    if not key:
        return jsonify({"error": "DeepL key not set"}), 400
    
    try:
        is_free = key.endswith(":fx")
        api_url = "https://api-free.deepl.com/v2/translate" if is_free else "https://api.deepl.com/v2/translate"
        headers = {"Authorization": f"DeepL-Auth-Key {key}", "Content-Type": "application/json"}

        # Если есть контекст и переводим одно слово/фразу — используем XML trick:
        # оборачиваем слово в тег <w>, DeepL переводит всё предложение и возвращает
        # тег с уже переведённым словом в нужном контексте
        is_single = context and context != text
        if is_single:
            import re as re_mod
            # Экранируем спецсимволы для точного поиска в предложении
            escaped = re_mod.escape(text)
            tagged_context = re_mod.sub(escaped, f'<w>{text}</w>', context, count=1)
            
            payload = {
                "text": [tagged_context],
                "source_lang": "EN",
                "target_lang": "RU",
                "tag_handling": "xml"
            }
            response = requests.post(api_url, headers=headers, json=payload, timeout=10)
            
            if response.status_code == 200:
                result = response.json()
                translated = result['translations'][0]['text']
                
                # Извлекаем перевод слова из тега <w>...</w>
                m = re_mod.search(r'<w>(.*?)</w>', translated)
                word_trans = m.group(1) if m else None
                
                # Перевод контекста — убираем теги
                context_trans = re_mod.sub(r'</?w>', '', translated)
                
                if word_trans:
                        add_deepl_chars(len(tagged_context))
                        return jsonify({"translation": word_trans, "context_translation": context_trans})
            # Если что-то пошло не так — падаем на обычный перевод ниже

        # Обычный перевод (выделение фразы или fallback)
        payload = {"text": [text], "source_lang": "EN", "target_lang": "RU"}
        response = requests.post(api_url, headers=headers, json=payload, timeout=10)
        
        if response.status_code == 403:
            return jsonify({"error": "invalid DeepL key"}), 401
        
        if response.status_code != 200:
            logger.error(f"DeepL status {response.status_code}: {response.text[:200]}")
            return jsonify({"error": f"DeepL error: {response.status_code}"}), 500
        
        result = response.json()
        add_deepl_chars(len(text))
        return jsonify({"translation": result['translations'][0]['text']})
    except Exception as e:
        logger.error(f"💥 DeepL error: {e}")
        return jsonify({"error": str(e)}), 500

@app.route('/check', methods=['POST'])
def check():
    """Проверяет, есть ли слово уже в Anki. Один запрос через multi action."""
    data = request.json
    word = data.get('word', '').strip()
    if not word:
        return jsonify({"error": "no word"}), 400
    try:
        res = requests.post(ANKI_URL, json={
            "action": "multi",
            "version": 6,
            "params": {"actions": [
                {"action": "findNotes", "params": {"query": f'Word:"{word}" deck:{DECK_NAME}'}},
            ]}
        }, timeout=3)
        results = res.json().get("result", [])
        note_ids = results[0] if results else []
        if not note_ids:
            return jsonify({"exists": False, "seen_count": 0})

        info_res = requests.post(ANKI_URL, json={
            "action": "notesInfo",
            "version": 6,
            "params": {"notes": [note_ids[0]]}
        }, timeout=3)
        fields = info_res.json()["result"][0]["fields"]
        seen = int(fields.get("SeenCount", {}).get("value", "1") or "1")
        return jsonify({"exists": True, "seen_count": seen})
    except Exception as e:
        logger.error(f"❌ /check error: {e}")
        return jsonify({"exists": False, "seen_count": 0})

@app.route('/config', methods=['GET', 'POST'])
def config():
    if request.method == 'POST':
        data = request.json
        if 'deepl_key' in data:
            set_deepl_key(data['deepl_key'])
        return jsonify({"status": "ok"})
    
    cfg = load_config()
    cfg['has_deepl_key'] = get_deepl_key() is not None
    chars = get_deepl_chars()
    cfg['deepl_chars_this_month'] = chars.get('count', 0)
    cfg['deepl_chars_month'] = chars.get('month', '')
    return jsonify(cfg)

if __name__ == '__main__':
    logger.info("🚀 СЕРВЕР ЗАПУЩЕН")
    ensure_model_fields()
    app.run(port=5005, threaded=True)