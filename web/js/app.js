// Digihub Frontend Catalog & Checkout Engine

let allProducts = [];
let selectedProduct = null;

// Format Rupiah
function formatIDR(num) {
  return 'Rp ' + Number(num || 0).toLocaleString('id-ID');
}

// Toast helper
function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2500);
}

// LocalStorage History Helpers (Delegates to unified DigihubOrders)
function getHistory() {
  const handler = window.DigihubOrders || window.StoreKuOrders;
  return handler ? handler.getOrders() : [];
}

function saveHistoryItem(item) {
  const handler = window.DigihubOrders || window.StoreKuOrders;
  if (handler) {
    handler.saveOrder(item);
  }
}

function updateHistoryBadge() {
  const handler = window.DigihubOrders || window.StoreKuOrders;
  if (handler) {
    handler.updateBadge();
  }
}

// Render Products Grid
function renderProducts(products) {
  const grid = document.getElementById('productsGrid');
  const empty = document.getElementById('emptyCatalog');
  const loading = document.getElementById('loadingCatalog');

  if (loading) loading.style.display = 'none';

  if (!products || products.length === 0) {
    if (grid) grid.style.display = 'none';
    if (empty) empty.style.display = 'block';
    return;
  }

  if (empty) empty.style.display = 'none';
  if (grid) grid.style.display = 'grid';

  grid.innerHTML = products.map(p => {
    const isOut = p.stock <= 0;
    const stockDisplay = isOut ? 'Habis' : (p.stockCapped ? '>1000 Stok' : `${p.stock} Stok`);
    const logoSrc = p.logo || 'https://via.placeholder.com/46?text=DC';

    return `
      <div class="product-card" onclick="window.location.href='/product?slug=' + encodeURIComponent('${escapeHtml(p.slug)}')" style="cursor: pointer;">
        <div>
          <div class="prod-header">
            <img src="${logoSrc}" alt="${escapeHtml(p.name)}" class="prod-logo" onerror="this.src='https://via.placeholder.com/46?text=DC'">
            <div class="prod-title-wrap">
              <h3 class="prod-title" title="${escapeHtml(p.name)}">${escapeHtml(p.name)}</h3>
              <div class="prod-category">${escapeHtml(p.type || 'Digital License')}</div>
            </div>
          </div>

          <div class="prod-tags-row">
            ${p.hasPromo ? `<span class="badge badge-promo">🔥 ${p.discountPercent}% OFF</span>` : ''}
            <span class="badge badge-stock ${isOut ? 'empty' : ''}">
              ● ${stockDisplay}
            </span>
            ${p.warranty ? `<span class="badge badge-warranty">🛡️ ${escapeHtml(p.warranty)}</span>` : ''}
            ${p.region ? `<span class="badge badge-region">🌐 ${escapeHtml(p.region)}</span>` : ''}
          </div>

          <div class="prod-desc">${escapeHtml(p.description || 'Lisensi akun premium langsung aktif.')}</div>
        </div>

        <div class="prod-footer">
          <div class="price-wrap">
            ${p.hasPromo ? `
              <div class="price-strike-wrap">
                <span class="price-original-striked">${formatIDR(p.referencePriceIDR)}</span>
                <span class="badge-discount-mini">-${p.discountPercent}%</span>
              </div>
              <span class="price-amount promo-highlight">${formatIDR(p.priceIDR)}</span>
            ` : `
              <span class="price-label">Mulai Dari</span>
              <span class="price-amount">${formatIDR(p.priceIDR)}</span>
            `}
          </div>
          <button class="btn-buy" onclick="event.stopPropagation(); window.location.href='/product?slug=' + encodeURIComponent('${escapeHtml(p.slug)}')">
            ${isOut ? 'Lihat Detail' : 'Beli Sekarang ➔'}
          </button>
        </div>
      </div>
    `;
  }).join('');
}

// Filter and Search
function applyFilterAndSearch() {
  const query = (document.getElementById('searchInput')?.value || '').trim().toLowerCase();
  const activeFilterBtn = document.querySelector('.tag-btn.active');
  const filterCat = activeFilterBtn?.dataset?.filter || 'all';

  let filtered = allProducts.filter(p => {
    const nameMatch = (p.name || '').toLowerCase().includes(query) ||
                      (p.description || '').toLowerCase().includes(query) ||
                      (p.type || '').toLowerCase().includes(query);

    if (!nameMatch) return false;

    if (filterCat === 'all') return true;
    if (filterCat === 'ai') return /gemini|chatgpt|claude|lovable|elevenlabs/i.test(p.name + p.description);
    if (filterCat === 'vpn') return /vpn|nord|shield/i.test(p.name + p.description);
    if (filterCat === 'stream') return /youtube|netflix|spotify|media/i.test(p.name + p.description);
    if (filterCat === 'edu') return /coursera|duolingo|adobe|notion/i.test(p.name + p.description);

    return true;
  });

  renderProducts(filtered);
}

// Checkout Modal Handling
function openCheckoutModal(slug) {
  selectedProduct = allProducts.find(p => p.slug === slug);
  if (!selectedProduct) return;

  const modal = document.getElementById('checkoutModal');
  document.getElementById('mProdName').textContent = selectedProduct.name;
  document.getElementById('mProdType').textContent = selectedProduct.type || 'Digital License';
  document.getElementById('mProdLogo').src = selectedProduct.logo || 'https://via.placeholder.com/40';

  const qtyInput = document.getElementById('qtyInput');
  qtyInput.value = 1;
  qtyInput.max = selectedProduct.stock > 0 ? selectedProduct.stock : 100;

  // Prefill email if previously used
  const savedEmail = localStorage.getItem('digihub_saved_email') || localStorage.getItem('storeku_saved_email') || '';
  const emailInput = document.getElementById('emailInput');
  if (emailInput && !emailInput.value) {
    emailInput.value = savedEmail;
  }

  // Render wholesale tiers if available
  const tierContainer = document.getElementById('tierPillsContainer');
  if (selectedProduct.tiers && selectedProduct.tiers.length > 0) {
    tierContainer.innerHTML = selectedProduct.tiers.map(t => {
      const range = t.maxQty ? `${t.minQty}–${t.maxQty} pcs` : `${t.minQty}+ pcs`;
      return `<span class="tier-pill">${range}: ${formatIDR(t.priceIDR)}</span>`;
    }).join(' ');
  } else {
    tierContainer.innerHTML = '';
  }

  updateCheckoutCalculations();
  modal.classList.add('open');
  if (emailInput && !emailInput.value) {
    setTimeout(() => emailInput.focus(), 150);
  }
}

function closeCheckoutModal() {
  document.getElementById('checkoutModal')?.classList.remove('open');
  selectedProduct = null;
}

function updateCheckoutCalculations() {
  if (!selectedProduct) return;

  const qty = parseInt(document.getElementById('qtyInput').value, 10) || 1;
  let unitPriceIDR = selectedProduct.priceIDR;

  // Check tiers for quantity
  if (selectedProduct.tiers && selectedProduct.tiers.length > 0) {
    for (const t of selectedProduct.tiers) {
      if (qty >= t.minQty && (t.maxQty === null || qty <= t.maxQty)) {
        unitPriceIDR = t.priceIDR;
        break;
      }
    }
  }

  const total = unitPriceIDR * qty;

  document.getElementById('mUnitPrice').textContent = formatIDR(unitPriceIDR);
  document.getElementById('mQtyDisplay').textContent = `x ${qty}`;
  document.getElementById('mTotalPrice').textContent = formatIDR(total);
}

// Process Checkout submission
async function submitCheckout() {
  if (!selectedProduct) return;

  const btn = document.getElementById('btnConfirmCheckout');
  const qty = parseInt(document.getElementById('qtyInput').value, 10) || 1;
  const email = (document.getElementById('emailInput')?.value || '').trim();
  const contact = (document.getElementById('contactInput')?.value || '').trim();

  if (!email || !email.includes('@') || !email.includes('.')) {
    alert('Silakan masukkan alamat email yang valid untuk pengiriman kode klaim!');
    document.getElementById('emailInput')?.focus();
    return;
  }

  btn.disabled = true;
  btn.innerText = 'Menyiapkan QRIS ShopeePay...';

  try {
    const res = await fetch('/api/checkout', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        product_slug: selectedProduct.slug,
        quantity: qty,
        customer_email: email,
        customer_contact: contact
      })
    });

    const data = await res.json();
    if (!data.success) {
      alert('Gagal membuat pesanan: ' + (data.error || 'Terjadi kesalahan'));
      btn.disabled = false;
      btn.innerText = 'Bayar Sekarang via QRIS ➔';
      return;
    }

    const order = data.data;

    // SAVE EMAIL & ORDER TO LOCAL STORAGE!
    localStorage.setItem('digihub_saved_email', email);
    localStorage.setItem('storeku_saved_email', email);
    const orderHandler = window.DigihubOrders || window.StoreKuOrders;
    if (orderHandler) {
      orderHandler.saveOrder({
        invoice_number: order.invoice_number,
        claim_code: order.claim_code,
        product_name: order.product_name,
        quantity: order.quantity,
        total_amount: order.total_amount,
        payment_status: 'PENDING',
        expires_at: order.expires_at,
        created_at: new Date().toISOString()
      });
    }

    // REDIRECT TO PAYMENT PAGE
    window.location.href = `/payment?inv=${order.invoice_number}`;
  } catch (err) {
    alert('Koneksi terputus: ' + err.message);
    btn.disabled = false;
    btn.innerText = 'Bayar Sekarang via QRIS ➔';
  }
}

// History Modal Render (Uses unified DigihubOrders)
function openHistoryModal() {
  const orderHandler = window.DigihubOrders || window.StoreKuOrders;
  if (orderHandler) {
    orderHandler.renderHistoryList('historyListContainer');
  }
  document.getElementById('historyModal')?.classList.add('open');
}

function closeHistoryModal() {
  document.getElementById('historyModal')?.classList.remove('open');
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>'"]/g, tag => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  }[tag] || tag));
}

// Lifecycle Init
document.addEventListener('DOMContentLoaded', async () => {
  updateHistoryBadge();

  // Load products
  try {
    const res = await fetch('/api/products');
    const data = await res.json();
    if (data.success && data.data) {
      allProducts = data.data;
      renderProducts(allProducts);
    } else {
      document.getElementById('loadingCatalog').textContent = 'Gagal memuat katalog: ' + (data.error || 'Server error');
    }
  } catch (e) {
    document.getElementById('loadingCatalog').textContent = 'Koneksi ke backend gagal: ' + e.message;
  }

  // Events
  document.getElementById('searchInput')?.addEventListener('input', applyFilterAndSearch);

  document.querySelectorAll('.tag-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('.tag-btn').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      applyFilterAndSearch();
    });
  });

  document.getElementById('qtyInput')?.addEventListener('input', updateCheckoutCalculations);
  document.getElementById('closeCheckoutBtn')?.addEventListener('click', closeCheckoutModal);
  document.getElementById('btnConfirmCheckout')?.addEventListener('click', submitCheckout);

  document.getElementById('openHistoryBtn')?.addEventListener('click', openHistoryModal);
  document.getElementById('closeHistoryBtn')?.addEventListener('click', closeHistoryModal);

  // Close modals on clicking overlay outside card
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) {
      closeCheckoutModal();
      closeHistoryModal();
    }
  });
});
