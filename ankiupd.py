#!/usr/bin/env python3
import urllib.request
import urllib.error
import json
import os
import sys

# ==========================================
# CONFIGURATION
# ==========================================
# Укажите адрес вашего облачного сервера (VPS) с anki_server
SERVER_URL = os.getenv("ANKI_SERVER_URL", "https://your-anki-server.com")
# Ваш секретный API_KEY от облачного сервера
API_KEY = os.getenv("ANKI_API_KEY", "your-secret-key")
# Адрес локального AnkiConnect (обычно не меняется)
LOCAL_ANKI = "http://127.0.0.1:8765"
# ==========================================

def request_json(url, method="GET", data=None, headers=None):
    if headers is None:
        headers = {}
    
    encoded_data = None
    if data is not None:
        encoded_data = json.dumps(data).encode('utf-8')
        headers["Content-Type"] = "application/json"
        
    req = urllib.request.Request(url, data=encoded_data, headers=headers, method=method)
    
    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            body = response.read().decode('utf-8')
            if not body:
                return {}
            return json.loads(body)
    except urllib.error.HTTPError as e:
        print(f"HTTP Error {e.code}: {url}")
        print(e.read().decode('utf-8'))
        return None
    except Exception as e:
        print(f"Request failed ({url}): {e}")
        return None

def get_card_seen_count(word):
    """Ищет карточку в локальном Anki и возвращает (note_id, seen_count)."""
    res = request_json(LOCAL_ANKI, method="POST", data={
        "action": "findNotes",
        "version": 6,
        "params": {"query": f'Word:"{word}" deck:"WordsFromSafari"'}
    })
    if not res or not res.get("result"):
        return None, None

    note_id = res["result"][0]
    info_res = request_json(LOCAL_ANKI, method="POST", data={
        "action": "notesInfo",
        "version": 6,
        "params": {"notes": [note_id]}
    })
    if not info_res or not info_res.get("result"):
        return note_id, 1

    fields = info_res["result"][0]["fields"]
    seen_val = fields.get("SeenCount", {}).get("value", "1")
    try:
        seen = int(seen_val)
    except (ValueError, TypeError):
        seen = 1
    return note_id, seen

def increment_seen_count(note_id, current_count):
    """Обновляет поле SeenCount для карточки."""
    request_json(LOCAL_ANKI, method="POST", data={
        "action": "updateNoteFields",
        "version": 6,
        "params": {
            "note": {
                "id": note_id,
                "fields": {"SeenCount": str(current_count + 1)}
            }
        }
    })

def main():
    if SERVER_URL == "https://your-anki-server.com":
        print("Пожалуйста, откройте этот скрипт и настройте SERVER_URL и API_KEY.")
        sys.exit(1)

    print(f"Связываемся с сервером {SERVER_URL}...")
    headers = {"X-API-Key": API_KEY}
    
    # 1. Получаем буфер
    notes = request_json(f"{SERVER_URL}/buffer", headers=headers)
    
    if notes is None:
        print("Не удалось получить буфер.")
        sys.exit(1)
        
    if len(notes) == 0:
        print("Буфер пуст. Новых слов нет.")
        sys.exit(0)
        
    print(f"Найдено карточек в буфере: {len(notes)}")
    
    successful_ids = []
    
    # 2. Отправляем в локальный Anki
    for note in notes:
        buf_id = note.get("_buf_id")
        word = note.get("params", {}).get("note", {}).get("fields", {}).get("Word", "Unknown")
        
        # Удаляем _buf_id перед отправкой в Anki (AnkiConnect ругается на лишние поля)
        payload = note.copy()
        if "_buf_id" in payload:
            del payload["_buf_id"]
            
        audio_base64 = payload.pop("_audio_base64", None)
        if audio_base64:
            safe_word = "".join(c for c in word if c.isalnum() or c in (" ", "_", "-")).replace(" ", "_")
            filename = f"tts_{safe_word}_{buf_id}.mp3"
            
            media_payload = {
                "action": "storeMediaFile",
                "version": 6,
                "params": {
                    "filename": filename,
                    "data": audio_base64
                }
            }
            media_res = request_json(LOCAL_ANKI, method="POST", data=media_payload)
            if media_res and media_res.get("error") is None:
                if "params" in payload and "note" in payload["params"] and "fields" in payload["params"]["note"]:
                    payload["params"]["note"]["fields"]["Audio"] = f"[sound:{filename}]"
            else:
                print(f"(ОШИБКА TTS: {media_res.get('error') if media_res else 'no response'}) ", end="")

        print(f"Добавляем: {word}... ", end="")
        
        # Отправляем в AnkiConnect
        res = request_json(LOCAL_ANKI, method="POST", data=payload)
        
        if res is None:
            print("ОШИБКА (AnkiConnect недоступен. Anki открыт?)")
            continue
            
        error = res.get("error")
        if error is None:
            print("ОК")
            successful_ids.append(buf_id)
        elif "duplicate" in str(error).lower():
            note_id, seen = get_card_seen_count(word)
            if note_id:
                increment_seen_count(note_id, seen)
                print(f"ДУБЛИКАТ (обновлен SeenCount: {seen} -> {seen + 1})")
            else:
                print("ДУБЛИКАТ (уже есть в колоде)")
            successful_ids.append(buf_id) # Считаем успешным, чтобы убрать из буфера
        else:
            print(f"ОШИБКА ({error})")
            
    # 3. Удаляем успешно добавленные из буфера на сервере
    if successful_ids:
        print(f"\nОчищаем {len(successful_ids)} карточек из серверного буфера...")
        res = request_json(f"{SERVER_URL}/buffer", method="DELETE", data={"ids": successful_ids}, headers=headers)
        if res and res.get("status") == "success":
            print("Готово! Буфер очищен.")
        else:
            print("Не удалось очистить буфер на сервере.")
    else:
        print("\nНет новых успешных карточек для очистки.")

if __name__ == "__main__":
    main()
