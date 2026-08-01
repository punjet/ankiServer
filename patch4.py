with open("userscript.js", "r") as f:
    code = f.read()

# Add initialization log
code = code.replace("(() => {", "(() => {\n    console.log('[Apple LookUp Ultra] Скрипт инициализирован. Сервер:', 'https://your-server-url.com');")

# Add logging to gmPost
code = code.replace(
    'return new Promise((resolve) => {',
    'console.log(`[Apple LookUp Ultra] Отправка слова на перевод: "${text}"`);\n        return new Promise((resolve) => {'
)

code = code.replace(
    'const data = JSON.parse(res.responseText);',
    'const data = JSON.parse(res.responseText);\n                        console.log(`[Apple LookUp Ultra] Ответ от сервера /translate:`, data);'
)

# Add logging to /add
code = code.replace(
    'ev.target.disabled = true;',
    'ev.target.disabled = true;\n                console.log(`[Apple LookUp Ultra] Отправка слова в Anki буфер: "${word}"`);'
)
code = code.replace(
    'onload: (res) => {',
    'onload: (res) => {\n                        console.log(`[Apple LookUp Ultra] Ответ от сервера /add:`, res.responseText);'
)

with open("userscript.js", "w") as f:
    f.write(code)

print("Patch 4 applied")
