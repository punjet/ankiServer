// ==UserScript==
// @name         Apple LookUp Ultra (Stable + Collapse + Battery Saver)
// @version      13.0
// @description  Стабильный переводчик: сворачивает длинный текст и бережет батарею
// @match        *://*/*
// @grant        GM_xmlhttpRequest
// @connect      127.0.0.1
// ==/UserScript==

(() => {
    const ANKI_CONFIG = {
        deckName: "WordsFromSafari",
        modelName: "WordsFromSafari",
        url: "http://127.0.0.1:5005/add"
    };

    const POPUP_ID = "apple-lookup-popup";
    const THROTTLE_MS = 120; 
    const MAX_CONTEXT_WORDS = 15; // Порог сворачивания контекста
    
    let lastWord = "";
    let isProcessing = false;
    let lastTime = 0;
    let ticking = false;

    const style = document.createElement('style');
    style.textContent = `
        #${POPUP_ID} {
            position: fixed; top: 0; left: 0; z-index: 2147483647;
            width: 300px; background: #1c1c1e; color: #fff;
            border: 1px solid #3a3a3c; border-radius: 12px;
            font-family: -apple-system, BlinkMacSystemFont, sans-serif;
            display: none; pointer-events: auto;
            box-shadow: 0 10px 30px rgba(0,0,0,0.5);
            will-change: transform;
            transform: translate3d(0,0,0);
            contain: content;
        }
        .apple-content { padding: 14px; }
        .apple-word { font-weight: 700; font-size: 17px; display: block; margin-bottom: 2px; }
        .apple-trans { font-size: 15px; color: #0A84FF; margin-bottom: 10px; font-weight: 500; }
        
        .apple-context-box {
            font-size: 12px; color: #a1a1a6; line-height: 1.4;
            padding-top: 8px; border-top: 1px solid #2c2c2e;
        }

        /* Стили для сворачивания */
        .apple-context-text { display: none; margin-top: 5px; }
        .apple-context-text.expanded { display: block; }
        
        .apple-expand-btn {
            background: none; border: none; color: #0A84FF; 
            font-size: 11px; padding: 4px 0; cursor: pointer; text-decoration: none;
            display: block; font-weight: 500;
        }

        .apple-context-trans { color: #32D74B; font-style: italic; display: block; margin-top: 4px; }
        .apple-footer { padding: 10px; background: rgba(255, 255, 255, 0.03); border-radius: 0 0 12px 12px; }
        .apple-anki-btn { background: #0A84FF; color: #fff; border: none; width: 100%; padding: 9px; border-radius: 8px; cursor: pointer; font-weight: 600; }
        .btn-success { background: #32D74B !important; }
    `;
    document.head.appendChild(style);

    const popup = document.createElement("div");
    popup.id = POPUP_ID;
    document.body.appendChild(popup);

    async function translate(text) {
        if (!text) return "";
        try {
            const url = `https://translate.googleapis.com/translate_a/single?client=gtx&sl=auto&tl=ru&dt=t&q=${encodeURIComponent(text)}`;
            const res = await fetch(url);
            const data = await res.json();
            return data[0].map(t => t[0]).join("");
        } catch (e) { return ""; }
    }

    document.addEventListener("mousemove", (e) => {
        if (!e.shiftKey) {
            if (popup.style.display === "block" && !popup.matches(':hover')) {
                popup.style.display = "none";
                lastWord = "";
            }
            return;
        }

        const now = Date.now();
        if (now - lastTime < THROTTLE_MS) return;
        lastTime = now;

        if (!ticking) {
            window.requestAnimationFrame(() => {
                checkUnderCursor(e);
                ticking = false;
            });
            ticking = true;
        }
    }, { passive: true });

    function checkUnderCursor(e) {
        if (isProcessing) return;

        if (popup.style.display === "block") {
            const r = popup.getBoundingClientRect();
            if (e.clientX > r.left - 40 && e.clientX < r.right + 40 && e.clientY > r.top - 40 && e.clientY < r.bottom + 40) return;
        }

        const range = document.caretRangeFromPoint(e.clientX, e.clientY);
        if (!range || range.startContainer.nodeType !== Node.TEXT_NODE) return;

        const text = range.startContainer.textContent;
        const offset = range.startOffset;

        const leftMatch = text.slice(0, offset).match(/[a-zA-Z0-9А-Яа-я']+$/);
        const rightMatch = text.slice(offset).match(/^[a-zA-Z0-9А-Яа-я']+/);
        const word = (leftMatch ? leftMatch[0] : "") + (rightMatch ? rightMatch[0] : "");

        if (word.length < 2 || word === lastWord) return;

        processWord(word, range.startContainer, e.clientX, e.clientY);
    }

    async function processWord(word, node, mouseX, mouseY) {
        isProcessing = true;
        lastWord = word;

        const fullText = node.parentElement ? node.parentElement.innerText : node.textContent;
        const sentences = fullText.split(/[.!?]\s/);
        const context = (sentences.find(s => s.includes(word)) || word).trim();
        const wordCount = context.split(/\s+/).length;

        const posX = Math.max(10, Math.min(mouseX - 145, window.innerWidth - 305));
        const posY = (mouseY > 300) ? mouseY - 15 : mouseY + 25;
        const shiftY = (mouseY > 300) ? "-100%" : "0";
        
        popup.style.transform = `translate3d(${posX}px, ${posY}px, 0) translateY(${shiftY})`;
        popup.innerHTML = `<div class="apple-content" style="text-align:center">...</div>`;
        popup.style.display = "block";

        try {
            const [wordTrans, contextTrans] = await Promise.all([
                translate(word),
                translate(context)
            ]);

            const isLong = wordCount > MAX_CONTEXT_WORDS;

            popup.innerHTML = `
                <div class="apple-content">
                    <span class="apple-word">${word}</span>
                    <div class="apple-trans">${wordTrans}</div>
                    <div class="apple-context-box">
                        <strong>Context:</strong>
                        ${isLong ? `<button class="apple-expand-btn" id="ctx-toggle">▼ Show full context (${wordCount} words)</button>` : ''}
                        <div class="apple-context-text ${!isLong ? 'expanded' : ''}" id="ctx-text">
                            ${context}
                            <span class="apple-context-trans">${contextTrans}</span>
                        </div>
                    </div>
                </div>
                <div class="apple-footer">
                    <button class="apple-anki-btn" id="anki-add">Add to Anki</button>
                </div>
            `;

            if (isLong) {
                document.getElementById("ctx-toggle").onclick = () => {
                    const t = document.getElementById("ctx-text");
                    const isExp = t.classList.toggle("expanded");
                    document.getElementById("ctx-toggle").innerText = isExp ? "▲ Hide context" : `▼ Show full context (${wordCount} words)`;
                };
            }

            document.getElementById("anki-add").onclick = (ev) => {
                ev.target.innerText = "Saving...";
                GM_xmlhttpRequest({
                    method: "POST",
                    url: ANKI_CONFIG.url,
                    headers: { "Content-Type": "application/json" },
                    data: JSON.stringify({
                        action: "addNote",
                        params: {
                            note: {
                                deckName: ANKI_CONFIG.deckName,
                                modelName: ANKI_CONFIG.modelName,
                                fields: {
                                    Word: word,
                                    WordTranslation: wordTrans,
                                    Context: context,
                                    ContextTranslation: contextTrans
                                }
                            }
                        }
                    }),
                    onload: (res) => {
                        const data = JSON.parse(res.responseText);
                        if (data.status === "saved" || !data.error) {
                            ev.target.innerText = "✓ Saved";
                            ev.target.className = "apple-anki-btn btn-success";
                        }
                    }
                });
            };
        } finally {
            isProcessing = false;
        }
    }

    window.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            popup.style.display = "none";
            lastWord = "";
        }
    });
})();