import json, os, requests
from flask import Flask, request, jsonify
from flask_cors import CORS

app = Flask(__name__)
CORS(app)

DB_DIR = os.path.expanduser("~/anki_server")
DB_FILE = os.path.join(DB_DIR, "anki_buffer.json")
ANKI_URL = "http://127.0.0.1:8765"

if not os.path.exists(DB_DIR):
    os.makedirs(DB_DIR)

def load_buffer():
    if os.path.exists(DB_FILE):
        with open(DB_FILE, "r", encoding="utf-8") as f:
            try: return json.load(f)
            except: return []
    return []

def save_buffer(buffer):
    with open(DB_FILE, "w", encoding="utf-8") as f:
        json.dump(buffer, f, ensure_ascii=False, indent=4)

@app.route('/add', methods=['POST'])
def add():
    data = request.json
    word = data.get('params', {}).get('note', {}).get('fields', {}).get('Word', 'Unknown')
    print(f"📥 Adding to buffer: {word}")
    buffer = load_buffer()
    buffer.append(data)
    save_buffer(buffer)
    return jsonify({"status": "saved"})

@app.route('/sync', methods=['POST'])
def sync():
    buffer = load_buffer()
    if not buffer: return jsonify({"error": "Buffer is empty"})
    
    success, failed, duplicates = 0, 0, 0
    remaining = []

    print(f"🔄 Starting sync of {len(buffer)} notes...")

    for note in buffer:
        word = note.get('params', {}).get('note', {}).get('fields', {}).get('Word', 'Unknown')
        try:
            response = requests.post(ANKI_URL, json=note, timeout=3)
            res = response.json()

            # ЛОГИКА УСПЕХА:
            # 1. Если Anki прислала просто число (ID карточки)
            # 2. Или если Anki прислала словарь с результатом и без ошибки
            is_id = isinstance(res, (int, float))
            is_standard_success = isinstance(res, dict) and res.get("error") is None

            if is_id or is_standard_success:
                print(f"✅ Successfully added: {word}")
                success += 1
                continue # Слово НЕ идет в remaining (удаляется из буфера)

            # ЛОГИКА ОШИБОК:
            if isinstance(res, dict) and res.get("error"):
                err = str(res["error"]).lower()
                if "duplicate" in err:
                    print(f"⚠️ Duplicate (removed from buffer): {word}")
                    duplicates += 1
                    continue # Дубликаты тоже удаляем из буфера, чтобы не спамить
                else:
                    print(f"❌ Anki rejected '{word}': {res['error']}")
                    failed += 1
                    remaining.append(note)
            else:
                print(f"❓ Unknown response for '{word}': {res}")
                remaining.append(note)
                failed += 1

        except Exception as e:
            print(f"❗ Connection error for '{word}': {e}")
            remaining.append(note)
            failed += 1

    save_buffer(remaining)
    return jsonify({
        "success": success,
        "failed": failed,
        "duplicates": duplicates,
        "remaining": len(remaining)
    })

if __name__ == '__main__':
    app.run(port=5005, threaded=True)