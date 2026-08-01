with open("userscript.js", "r") as f:
    code = f.read()

# Fix z-index overflow
code = code.replace('z-index: 2147483648', 'z-index: 2147483647')

with open("userscript.js", "w") as f:
    f.write(code)

print("Patch 3 applied")
