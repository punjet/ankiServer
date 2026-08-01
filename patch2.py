with open("userscript.js", "r") as f:
    code = f.read()

# Make it use popup.querySelector to avoid ID collisions
code = code.replace(
    'document.getElementById("settings-btn").onclick',
    'popup.querySelector(".apple-settings-btn").onclick'
)

# Use variables for GM_ functions
code = code.replace(
    'const ANKI_CONFIG',
    'const _gmGet = typeof GM_getValue === "function" ? GM_getValue : (k, d) => d;\n    const _gmSet = typeof GM_setValue === "function" ? GM_setValue : (k, v) => {};\n    const ANKI_CONFIG'
)

# Replace all GM_getValue with _gmGet
code = code.replace('GM_getValue(', '_gmGet(')
code = code.replace('GM_setValue(', '_gmSet(')

with open("userscript.js", "w") as f:
    f.write(code)

print("Patch 2 applied")
