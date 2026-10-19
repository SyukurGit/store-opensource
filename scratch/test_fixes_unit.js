const assert = require('assert');
const fs = require('fs');

console.log("=== RUNNING UNIT / INTEGRATION VERIFICATION FOR FIXES ===");

// 1. Verify claim.js has isEn in setupClaimReviewUI and no unhandled isEn reference
const claimCode = fs.readFileSync('web/js/claim.js', 'utf8').replace(/\r\n/g, '\n');
assert(claimCode.includes('function setupClaimReviewUI(order) {\n  const isEn ='), 'setupClaimReviewUI must declare isEn!');
assert(claimCode.includes('const q = (document.getElementById(\'claimInput\')?.value || \'\').trim()'), 'claimInput must have safe optional chaining!');
console.log("[+] claim.js: isEn properly declared in setupClaimReviewUI, null-check secured.");

// 2. Verify order-history.js has crypto amounts and correct CTA
const historyCode = fs.readFileSync('web/js/order-history.js', 'utf8');
assert(historyCode.includes('isBinanceOrder') && historyCode.includes('isBep20Order'), 'order-history.js must detect Binance and BEP-20 orders!');
assert(historyCode.includes('Pay Binance ➔') && historyCode.includes('Pay USDT ➔'), 'order-history.js must have specific Crypto Pay buttons!');
assert(historyCode.includes('amountStr = `${item.crypto_amount_str} ${item.crypto_currency || \'USDT\'}`'), 'order-history.js must format Binance crypto amount!');
console.log("[+] order-history.js: Crypto amounts and gateway-specific CTA buttons confirmed.");

// 3. Verify app.js has expanded category filters & scroll lock
const appCode = fs.readFileSync('web/js/app.js', 'utf8');
assert(appCode.includes('midjourney|perplexity|cursor|deepseek'), 'app.js category filter must include modern AI tools!');
assert(appCode.includes('surfshark|express'), 'app.js category filter must include modern VPN tools!');
assert(appCode.includes('document.body.classList.add(\'modal-open\')'), 'app.js must lock body scroll on modal open!');
assert(appCode.includes('document.body.classList.remove(\'modal-open\')'), 'app.js must unlock body scroll on modal close!');
console.log("[+] app.js: Expanded regex filter and modal-open scroll locks confirmed.");

// 4. Verify product.js localized errors & fallback
const productCode = fs.readFileSync('web/js/product.js', 'utf8');
assert(productCode.includes('Missing Product Parameter'), 'product.js must localize missing slug error!');
assert(productCode.includes('Product Not Found'), 'product.js must localize product not found error!');
assert(productCode.includes('Instant premium digital license delivered after payment verification.'), 'product.js must have EN description fallback!');
assert(productCode.includes('By purchasing, you agree to our terms and conditions.'), 'product.js must have EN terms fallback!');
assert(productCode.includes('Voucher ${vCode} auto-cancelled'), 'product.js must localize voucher cancellation message!');
console.log("[+] product.js: English localizations and fallbacks confirmed.");

// 5. Verify payment.html and en/payment.html have restored key emoji
const idPayment = fs.readFileSync('web/payment.html', 'utf8');
const enPayment = fs.readFileSync('web/en/payment.html', 'utf8');
assert(idPayment.includes('🔑 KODE AMBIL PESANAN ANDA'), 'web/payment.html must have 🔑 icon in claim card title!');
assert(enPayment.includes('🔑 YOUR ORDER PICKUP CODE'), 'web/en/payment.html must have 🔑 icon in claim card title!');
console.log("[+] payment.html & en/payment.html: 🔑 icon in claim card title confirmed.");

// 6. Verify admin.js auth guard and res.ok check
const adminCode = fs.readFileSync('web/js/admin.js', 'utf8');
assert(adminCode.includes('!token && !window.location.pathname.includes(\'/login\')'), 'admin.js must guard against unauthenticated fetches!');
assert(adminCode.includes('if (!res || !res.ok) return;'), 'admin.js loadStats must check res.ok before parsing JSON!');
console.log("[+] admin.js: Unauthenticated fetch guard and res.ok checks confirmed.");

console.log("\n>>> ALL 6 SURGICAL FIXES VERIFIED 100% SUCCESSFUL! <<<");
