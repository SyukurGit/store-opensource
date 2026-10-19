import asyncio
import http.server
import socketserver
import threading
import os
from playwright.async_api import async_playwright

os.chdir("C:/laragon/www/storeku/web")

with open("css/style.css", "r", encoding="utf-8") as f:
    orig = f.read()

# Notice where .qris-notice-point is
fixed = orig.replace(".qris-notice-point {", "}\n\n.qris-notice-point {", 1)
with open("css/style.test.css", "w", encoding="utf-8") as f:
    f.write(fixed)

with open("index.html", "r", encoding="utf-8") as f:
    orig_idx = f.read()

test_idx = orig_idx.replace("href=\"/css/style.css?v=5.9.20\"", "href=\"/css/style.test.css?v=1\"")
with open("test_index.html", "w", encoding="utf-8") as f:
    f.write(test_idx)

Handler = http.server.SimpleHTTPRequestHandler
httpd = socketserver.TCPServer(("127.0.0.1", 9993), Handler)
t = threading.Thread(target=httpd.serve_forever)
t.daemon = True
t.start()

async def run():
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        context = await browser.new_context(viewport={"width": 375, "height": 667})
        page = await context.new_page()
        await page.goto("http://127.0.0.1:9993/test_index.html")
        await page.wait_for_timeout(500)
        
        info = await page.evaluate("""() => {
            const sheet = Array.from(document.styleSheets).find(s => s.href && s.href.includes("style.test.css"));
            const ruleCount = sheet ? sheet.cssRules.length : 0;
            const scrollW = document.documentElement.scrollWidth;
            const innerW = window.innerWidth;
            
            const qrisImg = document.querySelector(".logo-qris-img");
            const qRect = qrisImg ? qrisImg.getBoundingClientRect() : null;
            
            const binanceImg = document.querySelector(".logo-binance-img");
            const bRect = binanceImg ? binanceImg.getBoundingClientRect() : null;
            
            const usdtImg = document.querySelector(".logo-usdt-img");
            const uRect = usdtImg ? usdtImg.getBoundingClientRect() : null;
            
            const hamburger = document.getElementById("navHamburgerBtn");
            const hRect = hamburger ? hamburger.getBoundingClientRect() : null;
            
            return {
                ruleCount, scrollW, innerW,
                qris: qRect,
                binance: bRect,
                usdt: uRect,
                hamburger: hRect
            };
        }""")
        print("FIXED TEST RESULT:", info)
        await page.screenshot(path="../scratch/fixed_mobile_success.png")
        await browser.close()

try:
    asyncio.run(run())
finally:
    httpd.shutdown()
    if os.path.exists("css/style.test.css"):
        os.remove("css/style.test.css")
    if os.path.exists("test_index.html"):
        os.remove("test_index.html")
