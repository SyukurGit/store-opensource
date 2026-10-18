const assert = require('assert');

// Logic directly simulating web/js/app.js & web/js/product.js
function getStockBanner(p, isEn = false) {
  const isOut = p.stock <= 0 || !!p.isForceOutOfStock;
  const stockDisplay = isOut
    ? (isEn ? 'Out of Stock' : 'Habis')
    : (p.stockCapped ? (isEn ? '>1000 In Stock' : '>1000 Stok') : (isEn ? `${p.stock} In Stock` : `${p.stock} Stok`));
  return stockDisplay;
}

function getMaxAvailableQty(p) {
  if (p && p.stock > 0 && !p.stockCapped) {
    return Math.min(p.stock, 100);
  }
  return 100;
}

// Case 1: Manual Stock (e.g. Admin sets 5) -> StockCapped: false, Stock: 5
const manual5 = { stock: 5, stockCapped: false, isForceOutOfStock: false };
assert.strictEqual(getStockBanner(manual5, false), '5 Stok', 'ID: Manual 5 should show "5 Stok"');
assert.strictEqual(getStockBanner(manual5, true), '5 In Stock', 'EN: Manual 5 should show "5 In Stock"');
assert.strictEqual(getMaxAvailableQty(manual5), 5, 'Max qty must cap at 5');

// Case 2: Manual Stock via 3 Keys -> StockCapped: false, Stock: 3
const keys3 = { stock: 3, stockCapped: false, isForceOutOfStock: false };
assert.strictEqual(getStockBanner(keys3, false), '3 Stok', 'ID: Keys 3 should show "3 Stok"');
assert.strictEqual(getStockBanner(keys3, true), '3 In Stock', 'EN: Keys 3 should show "3 In Stock"');
assert.strictEqual(getMaxAvailableQty(keys3), 3, 'Max qty must cap at 3');

// Case 3: Unlimited Stock -> StockCapped: true, Stock: 9999
const unlim = { stock: 9999, stockCapped: true, isForceOutOfStock: false };
assert.strictEqual(getStockBanner(unlim, false), '>1000 Stok', 'ID: Unlimited should show ">1000 Stok"');
assert.strictEqual(getStockBanner(unlim, true), '>1000 In Stock', 'EN: Unlimited should show ">1000 In Stock"');
assert.strictEqual(getMaxAvailableQty(unlim), 100, 'Max qty for unlimited should be default 100');

// Case 4: Force Out of Stock -> Stock: 0, isForceOutOfStock: true
const forceOut = { stock: 0, stockCapped: false, isForceOutOfStock: true };
assert.strictEqual(getStockBanner(forceOut, false), 'Habis', 'ID: Force out should show "Habis"');
assert.strictEqual(getStockBanner(forceOut, true), 'Out of Stock', 'EN: Force out should show "Out of Stock"');

console.log('All 4 stock banner and max-qty contract tests PASSED successfully.');
