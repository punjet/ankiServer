import re

with open("userscript.js", "r") as f:
    code = f.read()

# 1. Add grants
code = code.replace("// @grant        GM_xmlhttpRequest", "// @grant        GM_xmlhttpRequest\n// @grant        GM_setValue\n// @grant        GM_getValue")

# 2. Update gmPost headers
code = code.replace(
    'headers: { "Content-Type": "application/json" },',
    'headers: { "Content-Type": "application/json", "X-API-Key": GM_getValue("server_api_key", "") },'
)

# 3. Update modal
modal_old = """    function showSettingsModal() {
        const modal = document.createElement("div");
        modal.className = "apple-modal";
        modal.innerHTML = `
            <div class="apple-modal-content">
                <h3>⚙️ DeepL API Key</h3>
                <p>DeepL понимает контекст и правильно переводит слова в предложениях (напр. "substantive" → "содержательные", а не "существенный").<br><br>
                1. Получите ключ: <a href="https://www.deepl.com/pro-api" target="_blank" style="color:#0A84FF">deepl.com/pro-api</a><br>
                2. Выполните в терминале:<br>
                <code style="background:#1c1c1e;padding:8px;display:block;margin:10px 0;border-radius:6px;font-size:11px">echo "ВАШ_КЛЮЧ" > ~/anki_server/.deepl_key</code></p>
                <div style="display:flex;justify-content:flex-end">
                    <button class="btn-save" id="modal-ok">Понятно</button>
                </div>
            </div>
        `;
        document.body.appendChild(modal);
        modal.querySelector("#modal-ok").onclick = () => modal.remove();
        modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
    }"""

modal_new = """    function showSettingsModal() {
        const currentKey = GM_getValue("server_api_key", "");
        const modal = document.createElement("div");
        modal.className = "apple-modal";
        modal.innerHTML = `
            <div class="apple-modal-content">
                <h3>⚙️ Настройки API</h3>
                <p>Введите API-ключ от вашего сервера (API_KEY в Coolify), чтобы скрипт мог безопасно делать переводы и добавлять карточки.</p>
                <input type="password" id="api-key-input" placeholder="Server API Key" value="${currentKey}">
                <div style="display:flex;justify-content:flex-end">
                    <button class="btn-cancel" id="modal-cancel">Отмена</button>
                    <button class="btn-save" id="modal-ok">Сохранить</button>
                </div>
            </div>
        `;
        document.body.appendChild(modal);
        modal.querySelector("#modal-cancel").onclick = () => modal.remove();
        modal.querySelector("#modal-ok").onclick = () => {
            GM_setValue("server_api_key", modal.querySelector("#api-key-input").value.trim());
            modal.remove();
        };
        modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
    }"""

code = code.replace(modal_old, modal_new)

with open("userscript.js", "w") as f:
    f.write(code)

print("Patched userscript.js successfully.")
