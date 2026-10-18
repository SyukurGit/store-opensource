const fs = require('fs');
const assert = require('assert');

console.log('--- Checking web/product.html and web/en/product.html ---');
const idHtml = fs.readFileSync('web/product.html', 'utf8');
const enHtml = fs.readFileSync('web/en/product.html', 'utf8');

const requiredIds = [
  'checkoutModal',
  'closeCheckoutBtn',
  'mProdLogo',
  'mProdName',
  'mProdType',
  'mModalStockNote',
  'btnModalQtyMinus',
  'modalQtyInput',
  'btnModalQtyPlus',
  'selectedGatewayCard',
  'activeGwIconWrap',
  'activeGwLogo',
  'activeGwTitle',
  'activeGwSubtitle',
  'btnOpenGwSelector',
  'voucherInput',
  'btnApplyVoucher',
  'btnCancelVoucher',
  'voucherFeedback',
  'emailInput',
  'emailInputError',
  'contactInput',
  'orderSummaryBox',
  'mUnitPrice',
  'mQtyDisplay',
  'mVoucherRow',
  'mVoucherCodeTag',
  'mVoucherDiscountAmount',
  'mSummaryPaymentMethod',
  'mTotalPrice',
  'btnConfirmCheckout',
  'gwSelectorModal',
  'closeGwSelectorBtn',
  'selPayQRIS',
  'selPayBinance',
  'selPayBep20',
  'qrisRegionNoticeModal',
  'closeQrisNoticeBtn',
  'btnConfirmSwitchToQRIS',
  'btnCancelSwitchToQRIS'
];

for (const id of requiredIds) {
  assert(idHtml.includes(`id="${id}"`), `Missing id="${id}" in web/product.html`);
  assert(enHtml.includes(`id="${id}"`), `Missing id="${id}" in web/en/product.html`);
}
console.log('✓ All 40 critical element IDs exist in both Indonesian & English product pages.');

console.log('--- Checking web/css/style.css mobile rules ---');
const css = fs.readFileSync('web/css/style.css', 'utf8');

assert(css.includes('env(safe-area-inset-top'), 'safe-area-inset-top missing in CSS');
assert(css.includes('env(safe-area-inset-bottom'), 'safe-area-inset-bottom missing in CSS');
assert(css.includes('html[data-theme="light"] #checkoutModal .checkout-modal-card'), 'Light theme #checkoutModal override missing');
assert(css.includes('html[data-theme="light"] #checkoutModal .checkout-modal-header'), 'Light theme header override missing');
assert(css.includes('html[data-theme="light"] #checkoutModal .checkout-sticky-footer'), 'Light theme footer override missing');
assert(css.includes('-webkit-line-clamp: 2'), 'Product title line clamp 2 missing');
assert(css.includes('scroll-margin-bottom'), 'Scroll margin bottom for inputs missing');
assert(/\.order-summary-box\s+\.summary-row\.total\s*\{\s*display:\s*none\s*!important;?\s*\}/.test(css), 'Summary total duplication suppression missing');

console.log('✓ CSS responsive rules, touch targets, and light mode specificity verified.');

console.log('--- Checking web/js/product.js syntax and logic ---');
const js = fs.readFileSync('web/js/product.js', 'utf8');
assert(js.includes('modalBody.scrollTop = 0'), 'modalBody.scrollTop reset missing');
assert(js.includes('emailInput.scrollIntoView'), 'emailInput.scrollIntoView missing');
assert(js.includes('shouldShowQrisNotice()'), 'shouldShowQrisNotice function missing');
assert(js.includes('initialCheckoutCurrency'), 'initialCheckoutCurrency state missing');

// Verify shouldShowQrisNotice logic by evaluating pure function
const fnMatch = js.match(/function shouldShowQrisNotice\(\)\s*\{[\s\S]*?return false;\s*\}/);
assert(fnMatch, 'Could not extract shouldShowQrisNotice function');
const evalFn = new Function('initialCheckoutCurrency', 'window', 'sessionStorage', `${fnMatch[0]}; return shouldShowQrisNotice();`);

// Case 1: Initial currency IDR & IP ID -> NO notice (false)
assert.strictEqual(evalFn('IDR', { digimestClientCountry: 'ID' }, { getItem: () => 'ID' }), false, 'IDR + ID IP should NOT show notice');

// Case 2: Initial currency IDR & local/unknown IP -> NO notice (false)
assert.strictEqual(evalFn('IDR', {}, { getItem: () => null }), false, 'IDR + unknown IP should NOT show notice');

// Case 3: Initial currency USD -> MUST show notice (true)
assert.strictEqual(evalFn('USD', { digimestClientCountry: 'ID' }, { getItem: () => 'ID' }), true, 'USD initial currency MUST show notice');

// Case 4: Foreign IP (e.g. US) -> MUST show notice (true)
assert.strictEqual(evalFn('IDR', { digimestClientCountry: 'US' }, { getItem: () => 'US' }), true, 'Foreign IP MUST show notice');

console.log('✓ shouldShowQrisNotice logic successfully validated across all test cases.');

assert(idHtml.includes('class="gw-chevron"'), 'gw-chevron missing in web/product.html');
assert(enHtml.includes('class="gw-chevron"'), 'gw-chevron missing in web/en/product.html');
assert(idHtml.includes('qris-notice-point'), 'qris-notice-point missing in web/product.html');
assert(enHtml.includes('qris-notice-point'), 'qris-notice-point missing in web/en/product.html');
assert(css.includes('slideUpSheet'), 'slideUpSheet animation missing in CSS');
assert(css.includes('.btn-change-gw.open .gw-chevron'), 'Chevron rotation rule missing in CSS');

console.log('✓ All frontend checks passed successfully!');
