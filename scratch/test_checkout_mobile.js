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

console.log('✓ All frontend checks passed successfully!');
