// ==UserScript==
// @name         Apple LookUp Ultra (Context-Aware + DeepL)
// @version      17.0
// @description  Context translation via your VPS server
// @match        *://*/*
// @grant        GM_xmlhttpRequest
// @connect      *
// ==/UserScript==

(() => {
    // ==========================================
    // INSERT YOUR API KEY AND SERVER URL HERE:
    const API_KEY = "YOUR_API_KEY_HERE";
    const SERVER_URL = "https://your-server-url.com";
    // ==========================================

    const ANKI_CONFIG = {
        deckName: "WordsFromSafari",
        modelName: "WordsFromSafari",
        url: SERVER_URL
    };

    const POPUP_ID = "apple-lookup-popup";
    const THROTTLE_MS = 80;
    const MAX_CONTEXT_WORDS = 15;

    let lastWord = "";
    let isProcessing = false;
    let lastTime = 0;
    let ticking = false;

    let translationCache = new Map();
    let ankiCheckCache = new Map();

    const STYLES = `
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
        .apple-in-anki { font-size: 11px; color: #636366; display: block; margin-bottom: 6px; }

        .apple-context-box {
            font-size: 12px; color: #a1a1a6; line-height: 1.4;
            padding-top: 8px; border-top: 1px solid #2c2c2e;
        }

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
        .btn-already { background: #636366 !important; }
        .btn-error { background: #FF3B30 !important; }
    `;

    const styleEl = document.createElement("style");
    styleEl.textContent = STYLES;
    document.head.appendChild(styleEl);

    const popup = document.createElement("div");
    popup.id = POPUP_ID;
    document.body.appendChild(popup);

    function gmPost(word, contextStr) {
        if (translationCache.has(word + contextStr)) {
            return Promise.resolve(translationCache.get(word + contextStr));
        }

        return new Promise((resolve) => {
            GM_xmlhttpRequest({
                method: "POST",
                url: `${ANKI_CONFIG.url}/translate`,
                headers: { "Content-Type": "application/json", "X-API-Key": API_KEY },
                data: JSON.stringify({ text: word, context: contextStr || "" }),
                onload: (res) => {
                    try {
                        const data = JSON.parse(res.responseText);
                        if (data.error) {
                            resolve({ wordTrans: `⚠️ ${data.error}`, contextTrans: "" });
                        } else {
                            const result = {
                                wordTrans: data.translation || "",
                                contextTrans: data.context_translation || data.translation || ""
                            };
                            translationCache.set(word + contextStr, result);
                            resolve(result);
                        }
                    } catch (e) {
                        resolve({ wordTrans: "⚠️ JSON parsing error", contextTrans: "" });
                    }
                },
                onerror: () => resolve({ wordTrans: "⚠️ Network error (Server unreachable?)", contextTrans: "" }),
                ontimeout: () => resolve({ wordTrans: "⚠️ Timeout", contextTrans: "" }),
                timeout: 10000
            });
        });
    }

    function gmCheck(word) {
        if (ankiCheckCache.has(word)) return Promise.resolve(ankiCheckCache.get(word));

        return new Promise((resolve) => {
            const query = `Word:"${word}" deck:${ANKI_CONFIG.deckName}`;
            GM_xmlhttpRequest({
                method: "POST",
                url: `http://127.0.0.1:8765`,
                headers: { "Content-Type": "application/json" },
                data: JSON.stringify({ action: "findNotes", version: 6, params: { query } }),
                onload: (res) => {
                    try {
                        const data = JSON.parse(res.responseText);
                        if (!data.result || data.result.length === 0) {
                            ankiCheckCache.set(word, { exists: false, seen_count: 0 });
                            return resolve({ exists: false, seen_count: 0 });
                        }
                        
                        GM_xmlhttpRequest({
                            method: "POST",
                            url: `http://127.0.0.1:8765`,
                            headers: { "Content-Type": "application/json" },
                            data: JSON.stringify({ action: "notesInfo", version: 6, params: { notes: [data.result[0]] } }),
                            onload: (res2) => {
                                try {
                                    const infoData = JSON.parse(res2.responseText);
                                    let seenCount = 1;
                                    if (infoData.result && infoData.result.length > 0) {
                                        const seenField = infoData.result[0].fields.SeenCount;
                                        if (seenField && seenField.value) {
                                            seenCount = parseInt(seenField.value, 10) || 1;
                                        }
                                    }
                                    const result = { exists: true, seen_count: seenCount };
                                    ankiCheckCache.set(word, result);
                                    resolve(result);
                                 } catch(e) {
                                    resolve({ exists: true, seen_count: 1 });
                                }
                            },
                            onerror: () => resolve({ exists: true, seen_count: 1 }),
                            ontimeout: () => resolve({ exists: true, seen_count: 1 })
                        });
                    } catch (e) {
                        resolve({ exists: false, seen_count: 0 });
                    }
                },
                onerror: () => resolve({ exists: false, seen_count: 0 }),
                ontimeout: () => resolve({ exists: false, seen_count: 0 }),
                timeout: 3000
            });
        });
    }

    function gmGrammar(text) {
        return new Promise((resolve) => {
            GM_xmlhttpRequest({
                method: "POST",
                url: `${ANKI_CONFIG.url}/grammar`,
                headers: { "Content-Type": "application/json", "X-API-Key": API_KEY },
                data: JSON.stringify({ text: text, source_url: window.location.href }),
                onload: (res) => {
                    try {
                        const data = JSON.parse(res.responseText);
                        if (data.error) {
                            resolve({ error: data.error, detail: data.detail });
                        } else {
                            resolve(data);
                        }
                    } catch (e) {
                        resolve({ error: "⚠️ JSON parsing error" });
                    }
                },
                onerror: () => resolve({ error: "⚠️ Network error" }),
                ontimeout: () => resolve({ error: "⚠️ Timeout" }),
                timeout: 30000
            });
        });
    }

    document.addEventListener("mousemove", (e) => {
        const isTriggerKey = e.shiftKey || e.ctrlKey || e.metaKey;
        if (!isTriggerKey) {
            if (popup.style.display === "block") {
                const r = popup.getBoundingClientRect();
                const CLOSE_DIST = 100;
                const farFromPopup = e.clientX < r.left - CLOSE_DIST || e.clientX > r.right + CLOSE_DIST
                                  || e.clientY < r.top - CLOSE_DIST || e.clientY > r.bottom + CLOSE_DIST;
                if (farFromPopup) {
                    popup.style.display = "none";
                    lastWord = "";
                }
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
            // Dont re-trigger if hovering inside popup
            if (e.clientX > r.left - 40 && e.clientX < r.right + 40 && e.clientY > r.top - 40 && e.clientY < r.bottom + 40) return;
        }

        const selection = window.getSelection();
        const selectedText = selection.toString().trim();

        let textToTranslate = "";
        let targetNode = null;

        if (selectedText.length > 0) {
            textToTranslate = selectedText;
            targetNode = selection.anchorNode || document.elementFromPoint(e.clientX, e.clientY);
        } else {
            const range = document.caretRangeFromPoint(e.clientX, e.clientY);
            if (!range || range.startContainer.nodeType !== Node.TEXT_NODE) return;

            const text = range.startContainer.textContent;
            const offset = range.startOffset;

            const leftMatch = text.slice(0, offset).match(/[a-zA-Z']+$/);
            const rightMatch = text.slice(offset).match(/^[a-zA-Z']+/);
            textToTranslate = (leftMatch ? leftMatch[0] : "") + (rightMatch ? rightMatch[0] : "");
            targetNode = range.startContainer;
        }

        if (textToTranslate.length < 2 || textToTranslate === lastWord) return;

        processWord(textToTranslate, targetNode, e.clientX, e.clientY, e.ctrlKey || e.metaKey);
    }

    async function processWord(word, node, mouseX, mouseY, isGrammarMode) {
        isProcessing = true;
        lastWord = word;

        let fullText = "";
        if (node && node.parentElement) {
            fullText = node.parentElement.innerText;
        } else if (node) {
            fullText = node.textContent;
        }

        let context = word;
        if (fullText && fullText.includes(word)) {
            const sentences = fullText.split(/(?<=[.!?])\s+/);
            const foundSentence = sentences.find(s => s.includes(word));
            if (foundSentence) context = foundSentence.trim();
        }

        const wordCount = context.split(/\s+/).length;

        const posX = Math.max(10, Math.min(mouseX - 145, window.innerWidth - 305));
        const posY = (mouseY > 300) ? mouseY - 15 : mouseY + 25;
        const shiftY = (mouseY > 300) ? "-100%" : "0";

        popup.style.transform = `translate3d(${posX}px, ${posY}px, 0) translateY(${shiftY})`;
        popup.innerHTML = `
            <div class="apple-content" style="text-align:center">⏳...</div>
        `;
        popup.style.display = "block";

        try {
            const isSingleWord = !word.includes(" ");

            const [result, ankiStatus] = await Promise.all([
                gmPost(word, isSingleWord ? context : null),
                isSingleWord ? gmCheck(word) : Promise.resolve({ exists: false, seen_count: 0 })
            ]);

            const displayWordTrans = result.wordTrans;
            const contextTrans = isSingleWord ? result.contextTrans : result.wordTrans;

            const isLong = wordCount > MAX_CONTEXT_WORDS;
            const alreadyInAnki = ankiStatus.exists;
            const seenCount = ankiStatus.seen_count || 0;

            popup.innerHTML = `
                <div class="apple-content">
                    <span class="apple-word">${word}</span>
                    ${alreadyInAnki ? `<span class="apple-in-anki">already in Anki · seen ${seenCount}×</span>` : ''}
                    <div class="apple-trans">${displayWordTrans}</div>
                    <div class="apple-context-box">
                        <strong>Context:</strong>
                        ${isLong ? `<button class="apple-expand-btn" id="ctx-toggle">▼ Show (${wordCount} words)</button>` : ''}
                        <div class="apple-context-text ${!isLong ? 'expanded' : ''}" id="ctx-text">
                            ${context}
                            <span class="apple-context-trans">${contextTrans}</span>
                        </div>
                    </div>
                </div>
                <div class="apple-footer">
                    ${alreadyInAnki
                        ? `<button class="apple-anki-btn btn-already" id="anki-add">Already know (+1)</button>`
                        : `<button class="apple-anki-btn" id="anki-add">Add to Anki</button>`
                    }
                    ${isGrammarMode 
                        ? `<button class="apple-anki-btn" id="grammar-check" style="margin-top: 5px; background: #FF9500;">Check for errors</button>` 
                        : ''
                    }
                </div>
            `;

            if (isGrammarMode) {
                document.getElementById("grammar-check").onclick = async (ev) => {
                    const btn = ev.target;
                    btn.innerText = "⏳ Checking...";
                    btn.disabled = true;
                    const res = await gmGrammar(word);
                    if (res.error) {
                        btn.innerText = "❌ " + res.error;
                        btn.className = "apple-anki-btn btn-error";
                    } else {
                        btn.innerText = `✓ Cards added: ${res.cards_added} (Errors: ${res.errors_found})`;
                        btn.className = "apple-anki-btn btn-success";
                    }
                };
            }

            if (isLong) {
                document.getElementById("ctx-toggle").onclick = () => {
                    const t = document.getElementById("ctx-text");
                    const isExp = t.classList.toggle("expanded");
                    document.getElementById("ctx-toggle").innerText = isExp ? "▲ Hide" : `▼ Show (${wordCount} words)`;
                };
            }

            document.getElementById("anki-add").onclick = (ev) => {
                ev.target.innerText = "Saving...";
                ev.target.disabled = true;
                GM_xmlhttpRequest({
                    method: "POST",
                    url: `${ANKI_CONFIG.url}/add`,
                    headers: { "Content-Type": "application/json", "X-API-Key": API_KEY },
                    data: JSON.stringify({
                        action: "addNote",
                        sourceUrl: window.location.href,
                        params: {
                            note: {
                                deckName: ANKI_CONFIG.deckName,
                                modelName: ANKI_CONFIG.modelName,
                                fields: {
                                    Word: word,
                                    WordTranslation: displayWordTrans,
                                    Context: context,
                                    ContextTranslation: contextTrans
                                }
                            }
                        }
                    }),
                    onload: (res) => {
                        try {
                            const data = JSON.parse(res.responseText);
                            if (data.status === "saved" || !data.error) {
                                ev.target.innerText = alreadyInAnki ? `✓ +1 (seen ${seenCount + 1}×)` : "✓ Added";
                                ev.target.className = "apple-anki-btn btn-success";
                                ankiCheckCache.delete(word);
                            } else {
                                ev.target.innerText = "Error: " + data.error;
                                ev.target.className = "apple-anki-btn btn-error";
                            }
                        } catch(e) {
                            ev.target.innerText = "Server response error";
                            ev.target.className = "apple-anki-btn btn-error";
                        }
                    },
                    onerror: () => {
                        ev.target.innerText = "Network failure";
                        ev.target.className = "apple-anki-btn btn-error";
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