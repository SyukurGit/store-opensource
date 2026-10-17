// Digimest Admin Engine (Multi-page Support)

let allAdminProducts = [];
let filteredAdminProducts = [];
let currentProductFilter = 'ALL';
let currentEditingSlug = null;

let currentOrderFilter = 'ALL';
let allAdminOrders = [];
let filteredAdminOrders = [];

function formatIDR(num) {
  return 'Rp ' + Number(num || 0).toLocaleString('id-ID');
}

function formatDate(dateStr) {
  if (!dateStr) return '-';
  const d = new Date(dateStr);
  return d.toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' });
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>'"]/g, tag => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  }[tag] || tag));
}

function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2500);
}

// -------------------------------------------------------------
// AUTH & FETCH HELPER
// -------------------------------------------------------------
function getAdminHeaders() {
  const token = localStorage.getItem('digimest_admin_token') || localStorage.getItem('Digimest_admin_token') || localStorage.getItem('storeku_admin_token');
  const headers = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

async function adminFetch(url, options = {}) {
  const combinedHeaders = {
    ...getAdminHeaders(),
    ...(options.headers || {})
  };
  const targetUrl = window.digimestApi ? window.digimestApi(url) : url;
  const res = await fetch(targetUrl, { ...options, headers: combinedHeaders });
  if (res.status === 401) {
    localStorage.removeItem('digimest_admin_token');
    localStorage.removeItem('Digimest_admin_token');
    localStorage.removeItem('storeku_admin_token');
    window.location.href = '/admin/login';
    return null;
  }
  return res;
}

// -------------------------------------------------------------
// 1. DASHBOARD & STATS LOGIC
// -------------------------------------------------------------
async function loadStats() {
  const statRev = document.getElementById('statRevenue');
  if (!statRev) return;

  try {
    const res = await adminFetch('/api/admin/stats');
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) return;

    const s = json.data;
    statRev.textContent = formatIDR(s.total_revenue_idr);
    document.getElementById('statPaidCount').textContent = s.paid_orders;

    const dcBalEl = document.getElementById('statDcBalance');
    if (dcBalEl) {
      dcBalEl.textContent = `$${(s.digitalcore_balance || 0).toFixed(2)}`;
      if (s.digitalcore_balance < 5.0) {
        dcBalEl.style.color = '#f87171';
        dcBalEl.title = '⚠️ Peringatan: Saldo bot menipis (< $5.00), segera lakukan top-up!';
      } else {
        dcBalEl.style.color = '#10b981';
      }
    }
    const dcTopEl = document.getElementById('statDcBalanceTop');
    if (dcTopEl) dcTopEl.textContent = `$${(s.digitalcore_balance || 0).toFixed(2)}`;

    const silvexBalEl = document.getElementById('statSilvexBalance');
    if (silvexBalEl) {
      silvexBalEl.textContent = `$${(s.silvex_balance || 0).toFixed(2)}`;
      if (s.silvex_balance < 3.0) {
        silvexBalEl.style.color = '#f87171';
        silvexBalEl.title = '⚠️ Peringatan: Saldo Silvex menipis (< $3.00), segera lakukan top-up!';
      } else {
        silvexBalEl.style.color = '#10b981';
      }
    }
    const silvexTopEl = document.getElementById('statSilvexBalanceTop');
    if (silvexTopEl) silvexTopEl.textContent = `$${(s.silvex_balance || 0).toFixed(2)}`;

    const silvexStatusEl = document.getElementById('statSilvexStatus');
    if (silvexStatusEl) {
      if (s.silvex_connected) {
        silvexStatusEl.textContent = 'Aktif';
        silvexStatusEl.style.color = '#34d399';
      } else if (s.silvex_error) {
        silvexStatusEl.textContent = 'Error';
        silvexStatusEl.style.color = '#f87171';
        silvexStatusEl.title = s.silvex_error;
      }
    }

    const badge = document.getElementById('apiStatusBadge');
    if (badge) {
      if (s.digitalcore_connected) {
        badge.className = 'badge badge-stock';
        badge.textContent = `● DC Terhubung ($${(s.digitalcore_balance || 0).toFixed(2)})`;
      } else if (s.digitalcore_error) {
        badge.className = 'badge badge-stock empty';
        badge.textContent = '● DC Error / Disconnected';
        badge.title = s.digitalcore_error;
      }
    }

    document.getElementById('statDcOrders').textContent = s.digitalcore_orders || '0';
    document.getElementById('statTodayRevenue').textContent = formatIDR(s.today_revenue_idr);
    document.getElementById('statTodayOrders').textContent = s.today_orders;
    document.getElementById('statPendingCount').textContent = s.pending_orders;
    document.getElementById('statFailedCount').textContent = s.failed_orders;
  } catch (err) {
    console.error('Failed to load stats:', err);
  }
}

// -------------------------------------------------------------
// 2. PRODUCT MANAGEMENT & OVERRIDES
// -------------------------------------------------------------
async function loadAdminProducts() {
  const tbody = document.getElementById('adminProductsTableBody');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: var(--text-muted);">Memuat katalog produk upstream...</td></tr>`;

  try {
    const res = await adminFetch('/api/admin/products');
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) {
      tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: #f87171;">Gagal memuat produk: ${json.error || 'Server error'}</td></tr>`;
      return;
    }

    allAdminProducts = json.data;
    updateProductCounters();
    applyProductFilter();

  } catch (err) {
    console.error('Failed to load admin products:', err);
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: #f87171;">Error: ${err.message}</td></tr>`;
  }
}

function updateProductCounters() {
  const total = allAdminProducts.length;
  const active = allAdminProducts.filter(p => p.is_active).length;
  const hidden = total - active;
  const custom = allAdminProducts.filter(p => p.is_customized).length;

  if (document.getElementById('cntTotalProds')) document.getElementById('cntTotalProds').textContent = total;
  if (document.getElementById('cntActiveProds')) document.getElementById('cntActiveProds').textContent = active;
  if (document.getElementById('cntHiddenProds')) document.getElementById('cntHiddenProds').textContent = hidden;
  if (document.getElementById('cntCustomProds')) document.getElementById('cntCustomProds').textContent = custom;
  if (document.getElementById('totalProdsBadge')) document.getElementById('totalProdsBadge').textContent = `● ${active} Aktif dari ${total} Upstream`;
}

function applyProductFilter() {
  const search = (document.getElementById('searchProdInput')?.value || '').trim().toLowerCase();

  filteredAdminProducts = allAdminProducts.filter(p => {
    // Search match
    const match = (p.name || '').toLowerCase().includes(search) ||
                  (p.original_name || '').toLowerCase().includes(search) ||
                  (p.slug || '').toLowerCase().includes(search) ||
                  (p.type || '').toLowerCase().includes(search);
    if (!match) return false;

    // Filter match
    if (currentProductFilter === 'ALL') return true;
    if (currentProductFilter === 'ACTIVE') return p.is_active;
    if (currentProductFilter === 'HIDDEN') return !p.is_active;
    if (currentProductFilter === 'CUSTOM') return p.is_customized;
    return true;
  });

  renderAdminProductsTable();
}

function renderAdminProductsTable() {
  const tbody = document.getElementById('adminProductsTableBody');
  if (!tbody) return;

  if (filteredAdminProducts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: var(--text-muted);">Tidak ada produk yang cocok dengan filter.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredAdminProducts.map(p => {
    const isCustomPrice = p.custom_price_idr > 0;
    const isCustomTitle = !!p.custom_name;
    const logoSrc = p.logo || 'https://via.placeholder.com/40?text=DC';

    return `
      <tr>
        <td>
          <div style="display: flex; align-items: center; gap: 10px;">
            <img src="${logoSrc}" alt="" style="width: 36px; height: 36px; border-radius: 8px; object-fit: cover; border: 1px solid var(--border-subtle);" onerror="this.src='https://via.placeholder.com/36?text=DC'">
            <div>
              <div style="font-weight: 700; color: #fff;">${escapeHtml(p.name)}</div>
              ${isCustomTitle ? `<div style="font-size: 0.72rem; color: var(--text-muted);">Asli: ${escapeHtml(p.original_name)}</div>` : ''}
              <div style="font-size: 0.72rem; color: #818cf8; font-family: var(--font-mono);">${escapeHtml(p.slug)}</div>
            </div>
          </div>
        </td>
        <td>
          <div style="font-weight: 600;">${p.stock} pcs ${p.custom_stock !== undefined && p.custom_stock >= 0 ? '<span class="badge-custom" style="font-size: 0.65rem; padding: 1px 6px; margin-left: 4px;">Kustom</span>' : ''}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted);">${escapeHtml(p.region || 'Global')} • ${escapeHtml(p.warranty || '-')}</div>
        </td>
        <td>
          <div style="font-family: var(--font-mono); color: #fbbf24; font-weight: 700;">$${(p.original_price_usd || 0).toFixed(2)}</div>
          <div style="font-size: 0.72rem; color: #94a3b8;">(Modal: ${formatIDR(p.modal_idr)})</div>
          ${p.surge_percent > 0 ? `<div style="font-size: 0.7rem; color: #f59e0b; font-weight: 600;">+${p.surge_percent.toFixed(1)}% naik</div>` : ''}
        </td>
        <td>
          <div style="font-weight: 700; color: #fff; font-family: var(--font-mono);">${formatIDR(p.reference_price_idr)}</div>
          <div style="font-size: 0.72rem; color: var(--text-muted);">
            ${isCustomPrice ? '<span class="badge-custom">Kustom</span>' : '<span class="badge-auto">Formula Env</span>'}
          </div>
        </td>
        <td>
          ${p.has_promo ? `
            <div style="font-weight: 700; color: #fbbf24; font-family: var(--font-mono);">${formatIDR(p.promo_price_idr)}</div>
            <div style="font-size: 0.72rem;"><span class="badge-discount-mini">-${p.discount_percent}% OFF</span></div>
          ` : `
            <span style="color: var(--text-muted); font-size: 0.85rem;">-</span>
          `}
        </td>
        <td>
          ${p.is_auto_suspended 
            ? `<span class="status-pill failed" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" title="Modal upstream melonjak melebihi ambang batas. Dinonaktifkan otomatis sampai diverifikasi admin.">⚠️ Disuspend (+${p.surge_percent.toFixed(0)}%)</span>` 
            : (!p.is_active 
                ? '<span class="status-pill failed">● Tersembunyi</span>' 
                : (p.is_force_out_of_stock
                    ? '<span class="status-pill warning" style="background: rgba(245,158,11,0.18); color: #fbbf24; border: 1px solid rgba(245,158,11,0.35);" title="Produk dipaksa habis secara manual oleh admin, tetap tampil di etalase tapi tombol beli mati.">⚡ Paksa Habis</span>'
                    : (p.stock <= 0
                        ? '<span class="status-pill" style="background: rgba(148,163,184,0.12); color: #94a3b8; border: 1px solid rgba(148,163,184,0.25);" title="Stok asli dari server DigitalCore kosong.">● Habis (Upstream)</span>'
                        : '<span class="status-pill paid">● Aktif</span>')))}
        </td>
        <td style="text-align: right;">
          <button class="btn-action-mini" onclick="openEditProductModal('${escapeHtml(p.slug)}')">
            ✏️ Edit
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

function openEditProductModal(slug) {
  const p = allAdminProducts.find(item => item.slug === slug);
  if (!p) return;

  currentEditingSlug = slug;

  document.getElementById('edProdLogo').src = p.logo || 'https://via.placeholder.com/48?text=DC';
  document.getElementById('edOriginalName').textContent = p.original_name;
  document.getElementById('edProdSlug').textContent = p.slug;
  document.getElementById('edProdUSD').textContent = `$${(p.original_price_usd || 0).toFixed(2)}`;
  document.getElementById('edProdModalIDR').textContent = `Modal: ${formatIDR(p.modal_idr)}`;
  document.getElementById('edProdEnvPrice').textContent = formatIDR(p.env_price_idr);

  document.getElementById('edIsActive').checked = p.is_active;
  const forceOutEl = document.getElementById('edIsForceOutOfStock');
  if (forceOutEl) forceOutEl.checked = !!p.is_force_out_of_stock;
  document.getElementById('edCustomName').value = p.custom_name || '';

  // Custom Type
  const edType = document.getElementById('edCustomType');
  if (edType) edType.value = p.custom_type || '';
  const defTypeEl = document.getElementById('edDefaultTypeNotice');
  if (defTypeEl) defTypeEl.textContent = p.original_type || p.type || 'Digital License';

  // Custom Region
  const edReg = document.getElementById('edCustomRegion');
  if (edReg) edReg.value = p.custom_region || '';
  const defRegEl = document.getElementById('edDefaultRegionNotice');
  if (defRegEl) defRegEl.textContent = p.original_region || p.region || 'Global';

  // Custom Warranty
  const edWar = document.getElementById('edCustomWarranty');
  if (edWar) edWar.value = p.custom_warranty || '';
  const defWarEl = document.getElementById('edDefaultWarrantyNotice');
  if (defWarEl) defWarEl.textContent = p.original_warranty || p.warranty || 'Garansi Toko';

  // Custom Stock
  const edStock = document.getElementById('edCustomStock');
  if (edStock) edStock.value = (p.custom_stock !== undefined && p.custom_stock >= 0) ? p.custom_stock : '';
  const defStockEl = document.getElementById('edDefaultStockNotice');
  if (defStockEl) defStockEl.textContent = `${p.original_stock !== undefined ? p.original_stock : p.stock} Stok`;

  // Pre-fill Harga Acuan with current active reference price
  const refPrice = p.reference_price_idr || p.calculated_price_idr;
  document.getElementById('edCustomPrice').value = refPrice;
  document.getElementById('edMinPriceNotice').textContent = formatIDR(p.env_price_idr);

  // Pre-fill Promo Price
  document.getElementById('edPromoPrice').value = p.promo_price_idr > 0 ? p.promo_price_idr : '';
  document.getElementById('edMinPromoNotice').textContent = formatIDR(p.modal_idr);

  document.getElementById('edCustomDesc').value = p.custom_description || '';
  document.getElementById('edCustomTerms').value = p.custom_terms || (p.terms || 'Membeli berarti setuju');

  document.getElementById('editProductModal')?.classList.add('open');
}

function closeEditProductModal() {
  document.getElementById('editProductModal')?.classList.remove('open');
  currentEditingSlug = null;
}

async function saveProductOverride() {
  if (!currentEditingSlug) return;
  const p = allAdminProducts.find(item => item.slug === currentEditingSlug);
  if (!p) return;

  const customPrice = parseInt(document.getElementById('edCustomPrice').value, 10) || 0;
  const promoPrice = parseInt(document.getElementById('edPromoPrice').value, 10) || 0;

  // Client-side validations
  if (customPrice > 0 && customPrice < p.env_price_idr) {
    alert(`Harga acuan (${formatIDR(customPrice)}) tidak boleh lebih rendah dari harga default sistem (${formatIDR(p.env_price_idr)})`);
    return;
  }

  if (promoPrice > 0) {
    if (promoPrice < p.modal_idr) {
      alert(`Harga promo (${formatIDR(promoPrice)}) tidak boleh lebih rendah dari modal toko (${formatIDR(p.modal_idr)})`);
      return;
    }
    const effectiveRef = customPrice > 0 ? customPrice : p.env_price_idr;
    if (promoPrice >= effectiveRef) {
      alert(`Harga promo (${formatIDR(promoPrice)}) harus lebih murah dari harga acuan (${formatIDR(effectiveRef)})`);
      return;
    }
  }

  const customStockVal = document.getElementById('edCustomStock') ? document.getElementById('edCustomStock').value.trim() : '';
  let customStock = -1;
  if (customStockVal !== '') {
    const parsedStock = parseInt(customStockVal, 10);
    if (!isNaN(parsedStock) && parsedStock >= 0) {
      customStock = parsedStock;
    }
  }

  const btn = document.getElementById('btnSaveProduct');
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  const body = {
    is_active: document.getElementById('edIsActive').checked,
    is_force_out_of_stock: document.getElementById('edIsForceOutOfStock') ? document.getElementById('edIsForceOutOfStock').checked : false,
    custom_name: (document.getElementById('edCustomName').value || '').trim(),
    custom_type: document.getElementById('edCustomType') ? document.getElementById('edCustomType').value.trim() : '',
    custom_region: document.getElementById('edCustomRegion') ? document.getElementById('edCustomRegion').value.trim() : '',
    custom_warranty: document.getElementById('edCustomWarranty') ? document.getElementById('edCustomWarranty').value.trim() : '',
    custom_stock: customStock,
    custom_price_idr: customPrice,
    promo_price_idr: promoPrice,
    custom_description: (document.getElementById('edCustomDesc').value || '').trim(),
    custom_terms: (document.getElementById('edCustomTerms').value || '').trim()
  };

  try {
    const res = await adminFetch(`/api/admin/products/${encodeURIComponent(currentEditingSlug)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Kustomisasi produk & harga berhasil disimpan! ✨');
      closeEditProductModal();
      await loadAdminProducts();
    } else {
      alert('Gagal menyimpan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Simpan Kustomisasi';
  }
}

async function resetProductOverride() {
  if (!currentEditingSlug) return;
  if (!confirm(`Reset kustomisasi produk "${currentEditingSlug}" ke pengaturan bawaan DigitalCore?`)) return;

  const btn = document.getElementById('btnResetProduct');
  btn.disabled = true;

  try {
    const res = await adminFetch(`/api/admin/products/${encodeURIComponent(currentEditingSlug)}/reset`, {
      method: 'POST'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Produk dikembalikan ke default DigitalCore 🔄');
      closeEditProductModal();
      await loadAdminProducts();
    } else {
      alert('Gagal mereset: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    btn.disabled = false;
  }
}

// -------------------------------------------------------------
// 3. ORDERS MANAGEMENT
// -------------------------------------------------------------
async function loadOrders() {
  const tbody = document.getElementById('ordersTableBody');
  if (!tbody) return;

  const isDashboard = window.location.pathname === '/admin' || window.location.pathname === '/admin/' || window.location.pathname.endsWith('index.html');
  const limit = isDashboard ? 5 : 100;

  tbody.innerHTML = `<tr><td colspan="9" style="text-align:center; padding: 2rem; color: var(--text-muted);">Memuat data pesanan...</td></tr>`;

  try {
    const statusParam = (currentOrderFilter === 'HOLD' || currentOrderFilter === 'ALL') ? '' : currentOrderFilter;
    const res = await adminFetch(`/api/admin/orders?limit=${limit}&status=${encodeURIComponent(statusParam)}`);
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) {
      tbody.innerHTML = `<tr><td colspan="9" style="text-align:center; padding: 2rem; color: #f87171;">Gagal memuat pesanan.</td></tr>`;
      return;
    }

    allAdminOrders = json.data;
    if (document.getElementById('totalOrdersBadge')) {
      document.getElementById('totalOrdersBadge').textContent = `● Total ${allAdminOrders.length} Pesanan Terdata`;
    }

    applyOrderFilter();

  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="9" style="text-align:center; padding: 2rem; color: #f87171;">Error: ${err.message}</td></tr>`;
  }
}

function applyOrderFilter() {
  const search = (document.getElementById('searchOrderInput')?.value || '').trim().toLowerCase();

  filteredAdminOrders = allAdminOrders.filter(o => {
    if (currentOrderFilter === 'HOLD' && o.fulfillment_status !== 'HOLD_PRICE_SPIKE' && o.fulfillment_status !== 'HOLD_INSUFFICIENT_BOT_BALANCE') {
      return false;
    }
    const match = (o.invoice_number || '').toLowerCase().includes(search) ||
                  (o.customer_email || '').toLowerCase().includes(search) ||
                  (o.customer_contact || '').toLowerCase().includes(search) ||
                  (o.claim_code || '').toLowerCase().includes(search) ||
                  (o.gateway_order_id || '').toLowerCase().includes(search) ||
                  (o.digitalcore_order_id || '').toLowerCase().includes(search) ||
                  (o.product_name || '').toLowerCase().includes(search);
    return match;
  });

  renderOrdersTable();
}

function renderOrdersTable() {
  const tbody = document.getElementById('ordersTableBody');
  if (!tbody) return;

  const isDashboard = window.location.pathname === '/admin' || window.location.pathname === '/admin/' || window.location.pathname.endsWith('index.html');
  const colSpan = isDashboard ? 7 : 9;

  if (filteredAdminOrders.length === 0) {
    tbody.innerHTML = `<tr><td colspan="${colSpan}" style="text-align:center; padding: 2rem; color: var(--text-muted);">Belum ada data pesanan.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredAdminOrders.map(o => {
    let payPillClass = 'pending';
    if (o.payment_status === 'PAID') payPillClass = 'paid';
    if (o.payment_status === 'EXPIRED') payPillClass = 'failed';

    let fulfillPillClass = 'pending';
    let fulfillLabel = o.fulfillment_status;
    let customFulfillStyle = '';

    if (o.fulfillment_status === 'SUCCESS') {
      fulfillPillClass = 'paid';
    } else if (o.fulfillment_status === 'FAILED') {
      fulfillPillClass = 'failed';
    } else if (o.fulfillment_status === 'HOLD_PRICE_SPIKE') {
      fulfillLabel = '⚠️ Ditahan (Harga)';
      customFulfillStyle = 'background: rgba(245,158,11,0.2); color: #fbbf24; border: 1px solid rgba(245,158,11,0.4);';
    } else if (o.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE') {
      fulfillLabel = '⚠️ Saldo Bot Kurang';
      customFulfillStyle = 'background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);';
    } else if (o.fulfillment_status === 'REFUNDED') {
      fulfillLabel = '💸 Refunded';
      customFulfillStyle = 'background: rgba(148,163,184,0.15); color: #94a3b8; border: 1px solid rgba(148,163,184,0.3);';
    } else if (o.fulfillment_status === 'PENDING_MANUAL_DELIVERY') {
      fulfillLabel = '⚠️ Butuh Kirim Key';
      customFulfillStyle = 'background: rgba(234,179,8,0.2); color: #facc15; border: 1px solid rgba(234,179,8,0.5); font-weight: 700;';
    }

    const isFulfillFailed = o.payment_status === 'PAID' && o.fulfillment_status === 'FAILED';
    const isHold = o.fulfillment_status === 'HOLD_PRICE_SPIKE' || o.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE';
    const isManualDeliver = o.fulfillment_status === 'PENDING_MANUAL_DELIVERY';
    const isPending = o.payment_status === 'PENDING';

    if (isDashboard) {
      return `
        <tr>
          <td>
            <div style="font-weight: 700; color: #fff; font-family: var(--font-mono);">${escapeHtml(o.invoice_number)}</div>
            <div style="font-size: 0.75rem; color: var(--text-muted);">${formatDate(o.created_at)}</div>
          </td>
          <td>
            <div style="font-weight: 600;">${escapeHtml(o.product_name)}</div>
            <div style="font-size: 0.78rem; color: var(--text-muted);">${o.quantity} item</div>
          </td>
          <td>
            <strong style="color: #10b981; font-family: var(--font-mono);">${formatIDR(o.total_amount)}</strong>
          </td>
          <td>
            <span class="status-pill ${payPillClass}">${o.payment_status}</span>
          </td>
          <td>
            <span class="status-pill ${fulfillPillClass}" style="${customFulfillStyle}">${fulfillLabel}</span>
          </td>
          <td>
            <code style="color: #a855f7; font-size: 0.82rem;">${escapeHtml(o.claim_code)}</code>
          </td>
          <td>
            <div style="display: flex; gap: 4px; flex-wrap: wrap;">
              <button class="btn-action-mini" onclick="viewOrderDetail('${escapeHtml(o.invoice_number)}')">Detail</button>
              ${isManualDeliver ? `<button class="btn-action-mini" style="background: rgba(234,179,8,0.25); color: #facc15; border: 1px solid rgba(234,179,8,0.6);" onclick="openManualDeliverModal('${escapeHtml(o.invoice_number)}')">🔑 Kirim Key</button>` : ''}
              ${isFulfillFailed ? `<button class="btn-retry" onclick="retryFulfillment('${escapeHtml(o.invoice_number)}')">Retry</button>` : ''}
              ${isHold ? `
                <button class="btn-action-mini" style="background: rgba(16,185,129,0.2); color: #34d399; border: 1px solid rgba(16,185,129,0.4);" onclick="forceFulfillOrder('${escapeHtml(o.invoice_number)}')">Beli</button>
                <button class="btn-action-mini" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" onclick="markRefundedOrder('${escapeHtml(o.invoice_number)}')">Refund</button>
              ` : ''}
              <button class="btn-action-mini" style="background: rgba(239,68,68,0.12); color: #f87171; border: 1px solid rgba(239,68,68,0.35);" onclick="deleteOrder('${escapeHtml(o.invoice_number)}')">Hapus</button>
            </div>
          </td>
        </tr>
      `;
    }

    return `
      <tr>
        <td>
          <div style="font-weight: 700; color: #fff; font-family: var(--font-mono);">${escapeHtml(o.invoice_number)}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted);">${formatDate(o.created_at)}</div>
        </td>
        <td>
          <div style="color: #93c5fd; font-size: 0.85rem; font-weight: 600;">${escapeHtml(o.customer_email || 'Tanpa Email')}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted);">${escapeHtml(o.customer_contact || '-')}</div>
        </td>
        <td>
          <div style="font-weight: 600;">${escapeHtml(o.product_name)}</div>
          <div style="font-size: 0.78rem; color: var(--text-muted);">${o.quantity} item</div>
        </td>
        <td>
          <strong style="color: #10b981; font-family: var(--font-mono);">${formatIDR(o.total_amount)}</strong>
        </td>
        <td>
          <span style="font-family: var(--font-mono); color: #fbbf24; font-weight: 700;">+${o.unique_code}</span>
        </td>
        <td>
          <span class="status-pill ${payPillClass}">${o.payment_status}</span>
        </td>
        <td>
          <span class="status-pill ${fulfillPillClass}" style="${customFulfillStyle}">${fulfillLabel}</span>
          ${o.fulfillment_error ? `<div style="font-size: 0.72rem; color: #f87171; max-width: 130px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" title="${escapeHtml(o.fulfillment_error)}">${escapeHtml(o.fulfillment_error)}</div>` : ''}
        </td>
        <td>
          <code style="color: #a855f7; font-size: 0.82rem;">${escapeHtml(o.claim_code)}</code>
        </td>
        <td style="text-align: right;">
          <div style="display: flex; gap: 4px; justify-content: flex-end; flex-wrap: wrap;">
            <button class="btn-action-mini" onclick="viewOrderDetail('${escapeHtml(o.invoice_number)}')">Detail</button>
            ${isManualDeliver ? `<button class="btn-action-mini" style="background: rgba(234,179,8,0.25); color: #facc15; border: 1px solid rgba(234,179,8,0.6);" onclick="openManualDeliverModal('${escapeHtml(o.invoice_number)}')">🔑 Kirim Key</button>` : ''}
            ${isFulfillFailed ? `<button class="btn-retry" onclick="retryFulfillment('${escapeHtml(o.invoice_number)}')">Retry Beli</button>` : ''}
            ${isHold ? `
              <button class="btn-action-mini" style="background: rgba(16,185,129,0.2); color: #34d399; border: 1px solid rgba(16,185,129,0.4);" onclick="forceFulfillOrder('${escapeHtml(o.invoice_number)}')">Beli Paksa</button>
              <button class="btn-action-mini" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" onclick="markRefundedOrder('${escapeHtml(o.invoice_number)}')">Refund</button>
            ` : ''}
            <button class="btn-action-mini" style="background: rgba(239,68,68,0.12); color: #f87171; border: 1px solid rgba(239,68,68,0.35);" onclick="deleteOrder('${escapeHtml(o.invoice_number)}')">Hapus</button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}


function viewOrderDetail(invoice) {
  const order = allAdminOrders.find(o => o.invoice_number === invoice);
  if (!order) return;

  let items = [];
  try {
    items = JSON.parse(order.delivered_items || '[]');
  } catch(e) {}

  const modal = document.getElementById('orderDetailModal');
  const container = document.getElementById('modalDetailContent');

  container.innerHTML = `
    <div style="display: flex; flex-direction: column; gap: 12px;">
      <div>
        <div style="font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase;">Invoice & Email Pembeli</div>
        <div style="font-weight: 700; color: #fff;">${escapeHtml(order.invoice_number)}</div>
        <div style="font-size: 0.85rem; color: #60a5fa;">${escapeHtml(order.customer_email || 'Tidak ada email')} • Kontak: ${escapeHtml(order.customer_contact || '-')}</div>
      </div>
      <div>
        <div style="font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase;">Produk & Pembayaran</div>
        <div>${escapeHtml(order.product_name)} • Qty: ${order.quantity} • Total: <strong style="color:#10b981;">${formatIDR(order.total_amount)}</strong></div>
      </div>
      <div>
        <div style="font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase;">Kode Klaim Rahasia</div>
        <code style="color: #a855f7; font-size: 1.05rem; font-weight: 700; font-family: var(--font-mono);">${escapeHtml(order.claim_code)}</code>
      </div>
      ${order.digitalcore_order_id ? `
      <div>
        <div style="font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase;">DigitalCore Upstream ID</div>
        <div>${escapeHtml(order.digitalcore_order_id)} ${order.digitalcore_view_url ? `<a href="${escapeHtml(order.digitalcore_view_url)}" target="_blank" style="color: var(--primary); margin-left: 8px;">Buka Web Upstream ↗</a>` : ''}</div>
      </div>` : ''}
      <div>
        <div style="font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; margin-bottom: 6px;">Lisensi / Serial Key Terkirim:</div>
        ${items.length === 0 ? `
          <div style="color: var(--text-muted); font-size: 0.85rem; margin-bottom: 8px;">Belum ada lisensi / serial key terkirim.</div>
          ${order.fulfillment_status === 'PENDING_MANUAL_DELIVERY' || order.payment_status === 'PAID' ? `
            <button class="btn-buy" style="padding: 0.4rem 1rem; font-size: 0.82rem;" onclick="openManualDeliverModal('${escapeHtml(order.invoice_number)}')">🔑 Kirim Lisensi / Key Manual Sekarang</button>
          ` : ''}
        ` : items.map(it => `
          <div class="license-item-box" style="margin-bottom: 6px;">
            <span>${escapeHtml(it)}</span>
          </div>
        `).join('')}
      </div>
      <div style="margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--border-subtle); display: flex; justify-content: flex-end;">
        <button class="btn-action-mini" style="background: rgba(239,68,68,0.15); color: #f87171; border: 1px solid rgba(239,68,68,0.4); padding: 6px 14px; font-weight: 600;" onclick="deleteOrder('${escapeHtml(order.invoice_number)}')">🗑️ Hapus Pesanan Permanen</button>
      </div>
    </div>
  `;

  modal?.classList.add('open');
}

async function deleteOrder(invoice) {
  if (!confirm(`Yakin ingin menghapus pesanan "${invoice}" secara permanen dari database?\n\nTindakan ini tidak dapat dibatalkan.`)) {
    return;
  }

  try {
    const res = await adminFetch(`/api/admin/orders/${encodeURIComponent(invoice)}`, {
      method: 'DELETE'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast(json.message || `Pesanan ${invoice} berhasil dihapus! 🗑️`);
      const modal = document.getElementById('orderDetailModal');
      if (modal && modal.classList.contains('open')) {
        modal.classList.remove('open');
      }
      await loadOrders();
    } else {
      alert('Gagal menghapus pesanan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan saat menghapus pesanan: ' + err.message);
  }
}

async function retryFulfillment(invoice) {
  if (!confirm(`Coba eksekusi ulang pembelian upstream untuk ${invoice}? Pastikan saldo bot Telegram sudah terisi.`)) return;

  try {
    const res = await adminFetch('/api/admin/retry', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ invoice })
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Fulfillment sukses! Lisensi berhasil didapat. 🎉');
      await loadOrders();
      await loadStats();
    } else {
      alert('Gagal: ' + (json.error || 'Upstream error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

async function forceFulfillOrder(invoice) {
  if (!confirm(`Konfirmasi pembelian paksa untuk invoice ${invoice}?\nSistem akan memotong saldo bot Telegram Anda sesuai harga modal saat ini.`)) return;

  try {
    const res = await adminFetch(`/api/admin/orders/${encodeURIComponent(invoice)}/force-fulfill`, {
      method: 'POST'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Pesanan berhasil dibeli dan dipenuhi! 🎉');
      await loadOrders();
      await loadStats();
    } else {
      alert('Gagal: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

async function manualPaidAndFulfill(invoice) {
  alert('Fitur pembayaran manual telah dinonaktifkan demi keselamatan dan integritas transaksi toko.');
}

async function markRefundedOrder(invoice) {
  if (!confirm(`Tandai pesanan ${invoice} sebagai telah di-refund manual?\nPastikan Anda sudah mengembalikan dana ke pembeli melalui CS.`)) return;

  try {
    const res = await adminFetch(`/api/admin/orders/${encodeURIComponent(invoice)}/mark-refunded`, {
      method: 'POST'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Pesanan berhasil ditandai sebagai REFUNDED. 💸');
      await loadOrders();
      await loadStats();
    } else {
      alert('Gagal: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

// -------------------------------------------------------------
// 4. SETTINGS LOGIC
// -------------------------------------------------------------
async function loadSettings() {
  const usdInput = document.getElementById('cfgUsdRate');
  if (!usdInput) return;

  try {
    const res = await adminFetch('/api/admin/settings');
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      usdInput.value = json.data.usd_rate || 18000;
      document.getElementById('cfgMarginPct').value = json.data.margin_percent || 25;
      document.getElementById('cfgMarginFixed').value = json.data.margin_fixed || 2000;
      const maxSurge = document.getElementById('cfgMaxSurgePct');
      if (maxSurge) maxSurge.value = json.data.max_surge_disable_percent || 50;
      const teleInput = document.getElementById('cfgTelegramUser');
      if (teleInput) teleInput.value = json.data.telegram_cs_username || 'Digimestsupport';

      const dcSwitch = document.getElementById('cfgDigitalCoreEnabled');
      if (dcSwitch) {
        const isDcOn = json.data.digitalcore_enabled !== 'false';
        dcSwitch.checked = isDcOn;
        updateDcSwitchBadge(isDcOn);
      }

      const silvexSwitch = document.getElementById('cfgSilvexEnabled');
      if (silvexSwitch) {
        const isSilvexOn = json.data.silvex_enabled !== 'false';
        silvexSwitch.checked = isSilvexOn;
        updateSilvexSwitchBadge(isSilvexOn);
      }
    }
  } catch (err) {
    console.error('Failed to load settings:', err);
  }
}

function updateDcSwitchBadge(isOn) {
  const badge = document.getElementById('badgeDcSwitchState');
  if (badge) {
    if (isOn) {
      badge.style.background = 'rgba(16, 185, 129, 0.2)';
      badge.style.color = '#34d399';
      badge.textContent = '● ON (AKTIF)';
    } else {
      badge.style.background = 'rgba(239, 68, 68, 0.2)';
      badge.style.color = '#f87171';
      badge.textContent = '● OFF (NONAKTIF)';
    }
  }
  const dcStatus = document.getElementById('settingsDcStatus');
  if (dcStatus) {
    if (isOn) {
      dcStatus.style.color = '#10b981';
      dcStatus.textContent = '● https://digitalcore.top (Tarik API Aktif)';
    } else {
      dcStatus.style.color = '#f87171';
      dcStatus.textContent = '○ Nonaktif (Toko Khusus Produk Mandiri)';
    }
  }
}

function updateSilvexSwitchBadge(isOn) {
  const badge = document.getElementById('badgeSilvexSwitchState');
  if (badge) {
    if (isOn) {
      badge.style.background = 'rgba(16, 185, 129, 0.2)';
      badge.style.color = '#34d399';
      badge.textContent = '● ON (AKTIF)';
    } else {
      badge.style.background = 'rgba(239, 68, 68, 0.2)';
      badge.style.color = '#f87171';
      badge.textContent = '● OFF (NONAKTIF)';
    }
  }
  const silvexStatus = document.getElementById('settingsSilvexStatus');
  if (silvexStatus) {
    if (isOn) {
      silvexStatus.style.color = '#a855f7';
      silvexStatus.textContent = '● silvex-ai.com (Tarik API Aktif)';
    } else {
      silvexStatus.style.color = '#f87171';
      silvexStatus.textContent = '○ Nonaktif (Silvex AI Dimatikan)';
    }
  }
}

async function saveSettings() {
  const btn = document.getElementById('btnSaveSettings');
  if (!btn) return;

  const usdRateVal = parseFloat(document.getElementById('cfgUsdRate')?.value) || 0;
  if (usdRateVal < 16000) {
    alert('Nilai kurs USD/USDT tidak boleh kurang dari Rp 16.000 untuk mencegah kerugian finansial toko!');
    document.getElementById('cfgUsdRate')?.focus();
    return;
  }

  btn.disabled = true;
  btn.innerText = 'Menyimpan...';

  const teleInput = document.getElementById('cfgTelegramUser');
  const dcSwitch = document.getElementById('cfgDigitalCoreEnabled');
  const silvexSwitch = document.getElementById('cfgSilvexEnabled');
  const body = {
    usd_rate: document.getElementById('cfgUsdRate').value,
    margin_percent: document.getElementById('cfgMarginPct').value,
    margin_fixed: document.getElementById('cfgMarginFixed').value,
    max_surge_disable_percent: document.getElementById('cfgMaxSurgePct')?.value || "50",
    telegram_cs_username: teleInput ? teleInput.value.trim().replace(/^@/, '') : "Digimestsupport",
    digitalcore_enabled: dcSwitch ? (dcSwitch.checked ? "true" : "false") : "true",
    silvex_enabled: silvexSwitch ? (silvexSwitch.checked ? "true" : "false") : "true"
  };

  try {
    const res = await adminFetch('/api/admin/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Pengaturan kurs, margin, dan CS berhasil disimpan! ✨');
    } else {
      alert('Gagal menyimpan: ' + (json.error || 'Server error'));
    }

  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.innerText = 'Simpan Perubahan';
  }
}

// -------------------------------------------------------------
// APP-SHELL NAVIGATION (SIDEBAR, DRAWER, & MOBILE BOTTOM NAV)
// -------------------------------------------------------------
function setupAppShellNavigation() {
  const currentPath = window.location.pathname.replace(/\.html$/, '');

  // 1. Sidebar Links active state
  document.querySelectorAll('.admin-sidebar-link').forEach(link => {
    const href = link.getAttribute('href');
    if (!href) return;
    const cleanHref = href.replace(/\.html$/, '');
    if (cleanHref === currentPath || (cleanHref !== '/admin' && currentPath === cleanHref)) {
      link.classList.add('active');
    } else if (cleanHref === '/admin' && (currentPath === '/admin' || currentPath === '/admin/')) {
      link.classList.add('active');
    }
  });

  // 2. Bottom Nav active state
  document.querySelectorAll('.bottom-nav-item').forEach(item => {
    const href = item.getAttribute('href');
    if (!href) return;
    const cleanHref = href.replace(/\.html$/, '');
    if (cleanHref === currentPath || (cleanHref === '/admin' && (currentPath === '/admin' || currentPath === '/admin/'))) {
      item.classList.add('active');
    }
  });

  // 3. Mobile Hamburger & Drawer Toggle
  const sidebar = document.getElementById('adminSidebar');
  const drawerOverlay = document.getElementById('adminDrawerOverlay');
  const hamburgerBtn = document.getElementById('adminHamburgerBtn');
  const bottomNavMenuBtn = document.getElementById('btnBottomNavMenu');

  function openSidebar() {
    sidebar?.classList.add('open');
    drawerOverlay?.classList.add('active');
    document.body.style.overflow = 'hidden';
  }

  function closeSidebar() {
    sidebar?.classList.remove('open');
    drawerOverlay?.classList.remove('active');
    document.body.style.overflow = '';
  }

  hamburgerBtn?.addEventListener('click', openSidebar);
  bottomNavMenuBtn?.addEventListener('click', openSidebar);
  drawerOverlay?.addEventListener('click', closeSidebar);

  // Close sidebar on link click on mobile
  document.querySelectorAll('.admin-sidebar-link').forEach(link => {
    link.addEventListener('click', () => {
      if (window.innerWidth < 1024) closeSidebar();
    });
  });
}

// -------------------------------------------------------------
// 5. INITIALIZATION & EVENT LISTENERS
// -------------------------------------------------------------
document.addEventListener('DOMContentLoaded', () => {
  setupAppShellNavigation();
  loadStats();
  loadAdminProducts();
  loadOrders();
  loadSettings();
  loadLastLoginInfo();
  loadDatabaseStats();
  setupDbMaintenanceUI();

  // Refresh Button
  document.getElementById('btnRefreshStats')?.addEventListener('click', () => {
    loadStats();
    loadAdminProducts();
    loadOrders();
    loadSettings();
    loadLastLoginInfo();
    loadDatabaseStats();
    if (typeof loadLocalProducts === 'function' && document.getElementById('localProductsTableBody')) {
      loadLocalProducts();
    }
    if (document.getElementById('reviewsPageWrap')) {
      loadEtalaseProducts();
      loadAdminReviews();
    }
    if (document.getElementById('adminSilvexTableBody')) {
      loadSilvexProducts();
    }
    showToast('Data diperbarui 🔄');
  });

  // Logout Button (Header & Sidebar)
  document.querySelectorAll('#btnLogout, #btnLogoutSidebar, .btn-admin-logout').forEach(btn => {
    btn.addEventListener('click', async () => {
      localStorage.removeItem('digimest_admin_token');
      localStorage.removeItem('Digimest_admin_token');
      localStorage.removeItem('storeku_admin_token');
      const logoutUrl = window.digimestApi ? window.digimestApi('/api/admin/logout') : '/api/admin/logout';
      try {
        await fetch(logoutUrl, { method: 'POST' });
      } catch (e) {}
      window.location.href = '/admin/login';
    });
  });

  // Settings Save & DigitalCore / Silvex Switch
  document.getElementById('btnSaveSettings')?.addEventListener('click', saveSettings);
  document.getElementById('cfgDigitalCoreEnabled')?.addEventListener('change', (e) => {
    updateDcSwitchBadge(e.target.checked);
  });
  document.getElementById('cfgSilvexEnabled')?.addEventListener('change', (e) => {
    updateSilvexSwitchBadge(e.target.checked);
  });

  // Product Filter Tabs & Search
  document.querySelectorAll('[data-pfilter]').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('[data-pfilter]').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      currentProductFilter = e.target.dataset.pfilter;
      applyProductFilter();
    });
  });

  document.getElementById('searchProdInput')?.addEventListener('input', applyProductFilter);

  // Edit Product Modal
  document.getElementById('closeEditModalBtn')?.addEventListener('click', closeEditProductModal);
  document.getElementById('btnCancelEdit')?.addEventListener('click', closeEditProductModal);
  document.getElementById('btnSaveProduct')?.addEventListener('click', saveProductOverride);
  document.getElementById('btnResetProduct')?.addEventListener('click', resetProductOverride);

  // Order Filters & Search
  document.querySelectorAll('[data-order-filter]').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('[data-order-filter]').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      currentOrderFilter = e.target.dataset.orderFilter;
      loadOrders();
    });
  });

  document.getElementById('searchOrderInput')?.addEventListener('input', applyOrderFilter);
  document.getElementById('closeDetailModalBtn')?.addEventListener('click', () => {
    document.getElementById('orderDetailModal')?.classList.remove('open');
  });

  // DigitalCore & Silvex Check Connection
  document.getElementById('btnCheckDcTop')?.addEventListener('click', checkDigitalCore);
  document.getElementById('btnCheckDcCard')?.addEventListener('click', checkDigitalCore);
  document.getElementById('btnCheckDcSettings')?.addEventListener('click', checkDigitalCore);
  document.getElementById('btnCheckSilvexTop')?.addEventListener('click', checkSilvexAI);
  document.getElementById('btnCheckSilvexCard')?.addEventListener('click', checkSilvexAI);
  document.getElementById('btnCheckSilvexPage')?.addEventListener('click', checkSilvexAI);
  document.getElementById('btnCheckSilvexSettings')?.addEventListener('click', checkSilvexAI);
  document.getElementById('closeDcModalBtn')?.addEventListener('click', () => {
    document.getElementById('dcCheckModal')?.classList.remove('open');
  });
  document.getElementById('closeDcModalBtn2')?.addEventListener('click', () => {
    document.getElementById('dcCheckModal')?.classList.remove('open');
  });

  // Last Login History Modal Handlers
  document.getElementById('btnOpenLoginHistory')?.addEventListener('click', openLoginHistoryModal);
  document.getElementById('closeLoginModalBtn')?.addEventListener('click', () => {
    document.getElementById('loginHistoryModal')?.classList.remove('open');
  });
  document.getElementById('closeLoginModalBtn2')?.addEventListener('click', () => {
    document.getElementById('loginHistoryModal')?.classList.remove('open');
  });

  // Security Page Handlers
  if (document.getElementById('secLogsTableBody')) {
    loadSecurityStats();
    loadSecurityLogs();

    document.getElementById('btnRefreshSecurity')?.addEventListener('click', () => {
      loadSecurityStats();
      loadSecurityLogs();
      showToast('Log keamanan diperbarui 🔄');
    });

    document.querySelectorAll('[data-sec-filter]').forEach(btn => {
      btn.addEventListener('click', (e) => {
        document.querySelectorAll('[data-sec-filter]').forEach(b => b.classList.remove('active'));
        e.target.classList.add('active');
        currentSecFilter = e.target.dataset.secFilter;
        secCurrentPage = 1;
        loadSecurityLogs();
      });
    });

    // Sort buttons
    const btnDesc = document.getElementById('btnSecOrderDesc');
    const btnAsc = document.getElementById('btnSecOrderAsc');
    if (btnDesc && btnAsc) {
      btnDesc.addEventListener('click', () => {
        if (secCurrentOrder === 'desc') return;
        secCurrentOrder = 'desc';
        btnDesc.style.background = 'rgba(99, 102, 241, 0.3)';
        btnDesc.style.color = '#fff';
        btnDesc.style.fontWeight = '600';
        btnAsc.style.background = 'transparent';
        btnAsc.style.color = 'var(--text-muted)';
        btnAsc.style.fontWeight = '500';
        secCurrentPage = 1;
        loadSecurityLogs();
      });

      btnAsc.addEventListener('click', () => {
        if (secCurrentOrder === 'asc') return;
        secCurrentOrder = 'asc';
        btnAsc.style.background = 'rgba(99, 102, 241, 0.3)';
        btnAsc.style.color = '#fff';
        btnAsc.style.fontWeight = '600';
        btnDesc.style.background = 'transparent';
        btnDesc.style.color = 'var(--text-muted)';
        btnDesc.style.fontWeight = '500';
        secCurrentPage = 1;
        loadSecurityLogs();
      });
    }

    // Limit select handler
    document.getElementById('secLimitSelect')?.addEventListener('change', (e) => {
      const val = parseInt(e.target.value, 10);
      if (!isNaN(val) && val > 0) {
        secCurrentLimit = val;
        secCurrentPage = 1;
        loadSecurityLogs();
      }
    });

    // Pagination buttons
    document.getElementById('btnSecPrev')?.addEventListener('click', () => {
      if (secCurrentPage > 1) {
        secCurrentPage--;
        loadSecurityLogs();
      }
    });

    document.getElementById('btnSecNext')?.addEventListener('click', () => {
      const totalPages = Math.max(1, Math.ceil(secTotalCount / secCurrentLimit));
      if (secCurrentPage < totalPages) {
        secCurrentPage++;
        loadSecurityLogs();
      }
    });

    let secSearchTimer = null;
    document.getElementById('secSearchInput')?.addEventListener('input', () => {
      clearTimeout(secSearchTimer);
      secCurrentPage = 1;
      secSearchTimer = setTimeout(loadSecurityLogs, 300);
    });
  }

  // Local Products Page Handlers
  if (document.getElementById('localProductsTableBody')) {
    loadLocalProducts();
    setupLocalProductsUI();
  }

  // Reviews & Etalase Page Handlers
  if (document.getElementById('reviewsPageWrap')) {
    setupReviewsPageUI();
    loadEtalaseProducts();
    loadAdminReviews();
  }

  // Multilingual Translations Page Handlers
  if (document.getElementById('translationsTableBody')) {
    loadTranslations();
    setupTranslationsUI();
  }

  // Silvex AI Products Page Handlers
  if (document.getElementById('adminSilvexTableBody')) {
    loadSilvexProducts();
    setupSilvexUI();
  }

  // Setup Manual Deliver UI (Orders & Dashboard)
  setupManualDeliverUI();

  // Close modals on backdrop click
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) {
      closeEditProductModal();
      closeEditSilvexModal();
      document.getElementById('orderDetailModal')?.classList.remove('open');
      document.getElementById('dcCheckModal')?.classList.remove('open');
      document.getElementById('loginHistoryModal')?.classList.remove('open');
      document.getElementById('dbMaintenanceModal')?.classList.remove('open');
      document.getElementById('localProductModal')?.classList.remove('open');
      document.getElementById('quickKeysModal')?.classList.remove('open');
      document.getElementById('manualDeliverModal')?.classList.remove('open');
      document.getElementById('editTransModal')?.classList.remove('open');
      document.getElementById('editSilvexModal')?.classList.remove('open');
    }
  });
});

// -------------------------------------------------------------
// 5. DIGITALCORE CONNECTION TEST ENGINE
// -------------------------------------------------------------
async function checkDigitalCore() {
  const modal = document.getElementById('dcCheckModal');
  const modalBody = document.getElementById('dcModalBody');
  const modalTitle = document.getElementById('dcModalTitle');
  const badge = document.getElementById('apiStatusBadge');

  if (modal && modalBody) {
    modalTitle.textContent = '🔍 Memeriksa Koneksi DigitalCore...';
    modalBody.innerHTML = `
      <div style="text-align: center; padding: 24px 0;">
        <div style="font-size: 2rem; margin-bottom: 8px;">⏳</div>
        <div style="font-weight: 700; color: #fff;">Sedang Menghubungi Server DigitalCore...</div>
        <div style="font-size: 0.8rem; color: var(--text-muted); margin-top: 4px;">Mengetes endpoint /api/user/me dengan API Key terpasang</div>
      </div>
    `;
    modal.classList.add('open');
  }

  try {
    const res = await adminFetch('/api/admin/digitalcore/check');
    if (!res) {
      if (modalBody) {
        modalBody.innerHTML = `<div style="color: #f87171;">Sesi admin tidak valid atau tidak dapat terhubung ke backend.</div>`;
      }
      return;
    }

    const json = await res.json();
    const data = json.data || {};

    if (json.success && data.success) {
      // Update balance & orders on screen immediately
      const dcBalEl = document.getElementById('statDcBalance');
      if (dcBalEl) {
        dcBalEl.textContent = `$${(data.balance || 0).toFixed(2)}`;
        dcBalEl.style.color = (data.balance >= 5.0) ? '#10b981' : '#f87171';
      }
      const dcOrdEl = document.getElementById('statDcOrders');
      if (dcOrdEl) {
        dcOrdEl.textContent = data.total_orders || '0';
      }
      if (badge) {
        badge.className = 'badge badge-stock';
        badge.textContent = `● DC Terhubung ($${(data.balance || 0).toFixed(2)})`;
      }

      showToast(`Koneksi DigitalCore Sukses! Saldo: $${(data.balance || 0).toFixed(2)}`);

      if (modalBody) {
        modalTitle.textContent = '✅ Koneksi DigitalCore Berhasil';
        modalBody.innerHTML = `
          <div style="background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.3); border-radius: 10px; padding: 14px; margin-bottom: 16px;">
            <div style="font-weight: 700; color: #34d399; font-size: 0.95rem; margin-bottom: 4px;">● Status: TERHUBUNG (HTTP ${data.http_status})</div>
            <div style="color: #cbd5e1; font-size: 0.82rem;">${escapeHtml(data.message || 'API Key valid dan upstream merespons normal.')}</div>
          </div>

          <table style="width: 100%; border-collapse: collapse; font-size: 0.84rem;">
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Saldo Bot Realtime</td>
              <td style="padding: 8px 0; font-weight: 800; font-family: var(--font-mono); font-size: 1.1rem; color: #10b981; text-align: right;">$${(data.balance || 0).toFixed(2)} USD</td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Total Order Upstream</td>
              <td style="padding: 8px 0; font-weight: 700; color: #fff; text-align: right;">${escapeHtml(data.total_orders || '0')} order</td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Base URL</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #93c5fd; text-align: right;">${escapeHtml(data.base_url || '-')}</td>
            </tr>
            <tr>
              <td style="padding: 8px 0; color: var(--text-muted);">API Key</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #cbd5e1; text-align: right;"><code>${escapeHtml(data.api_key_masked || '-')}</code></td>
            </tr>
          </table>

          ${data.raw_response ? `
            <div style="margin-top: 14px;">
              <div style="font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; font-weight: 700; margin-bottom: 4px;">Respon Mentah Upstream:</div>
              <pre style="background: rgba(0,0,0,0.4); border: 1px solid rgba(255,255,255,0.08); padding: 8px; border-radius: 6px; font-size: 0.75rem; color: #94a3b8; overflow-x: auto; max-height: 120px;">${escapeHtml(data.raw_response)}</pre>
            </div>
          ` : ''}
        `;
      }
    } else {
      if (badge) {
        badge.className = 'badge badge-stock empty';
        badge.textContent = '● DC Disconnected / Error';
      }

      showToast('Gagal terhubung ke DigitalCore.');

      if (modalBody) {
        modalTitle.textContent = '⚠️ Gagal Terhubung ke DigitalCore';
        modalBody.innerHTML = `
          <div style="background: rgba(239, 68, 68, 0.12); border: 1px solid rgba(239, 68, 68, 0.35); border-radius: 10px; padding: 14px; margin-bottom: 16px;">
            <div style="font-weight: 700; color: #f87171; font-size: 0.95rem; margin-bottom: 4px;">● Status: GAGAL (HTTP ${data.http_status || '-'})</div>
            <div style="color: #cbd5e1; font-size: 0.82rem;">${escapeHtml(data.message || json.error || 'Server DigitalCore menolak permintaan.')}</div>
          </div>

          <table style="width: 100%; border-collapse: collapse; font-size: 0.84rem;">
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Base URL</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #93c5fd; text-align: right;">${escapeHtml(data.base_url || '-')}</td>
            </tr>
            <tr>
              <td style="padding: 8px 0; color: var(--text-muted);">API Key Terpasang</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #cbd5e1; text-align: right;"><code>${escapeHtml(data.api_key_masked || '(kosong)')}</code></td>
            </tr>
          </table>

          <div style="margin-top: 14px; background: rgba(255,255,255,0.03); border-radius: 8px; padding: 10px; font-size: 0.78rem; color: #94a3b8;">
            💡 <strong>Panduan Solusi:</strong><br>
            1. Periksa file <code>.env</code> di VPS, pastikan <code>DIGITALCORE_API_KEY</code> sudah terisi.<br>
            2. Jika baru mengisi atau mengubah <code>.env</code>, jalankan <code>sudo systemctl restart storeku</code> di VPS agar aplikasi memuat key baru.
          </div>

          ${data.raw_response ? `
            <div style="margin-top: 14px;">
              <div style="font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; font-weight: 700; margin-bottom: 4px;">Respon Upstream:</div>
              <pre style="background: rgba(0,0,0,0.4); border: 1px solid rgba(255,255,255,0.08); padding: 8px; border-radius: 6px; font-size: 0.75rem; color: #f87171; overflow-x: auto; max-height: 120px;">${escapeHtml(data.raw_response)}</pre>
            </div>
          ` : ''}
        `;
      }
    }
  } catch (err) {
    if (modalBody) {
      modalTitle.textContent = '❌ Terjadi Kesalahan Jaringan';
      modalBody.innerHTML = `<div style="color: #f87171;">Tidak dapat mengirim request ke backend: ${escapeHtml(err.message)}</div>`;
    }
  }
}

async function checkSilvexAI() {
  const modal = document.getElementById('dcCheckModal');
  const modalBody = document.getElementById('dcModalBody');
  const modalTitle = document.getElementById('dcModalTitle');

  if (modal && modalBody) {
    modalTitle.textContent = '🔍 Memeriksa Koneksi Silvex AI...';
    modalBody.innerHTML = `
      <div style="text-align: center; padding: 24px 0;">
        <div style="font-size: 2rem; margin-bottom: 8px;">⏳</div>
        <div style="font-weight: 700; color: #fff;">Sedang Menghubungi Server Silvex AI...</div>
        <div style="font-size: 0.8rem; color: var(--text-muted); margin-top: 4px;">Mengetes endpoint /balance dengan x-api-key terpasang</div>
      </div>
    `;
    modal.classList.add('open');
  }

  try {
    const res = await adminFetch('/api/admin/silvex/check');
    if (!res) {
      if (modalBody) {
        modalBody.innerHTML = `<div style="color: #f87171;">Sesi admin tidak valid atau tidak dapat terhubung ke backend.</div>`;
      }
      return;
    }

    const json = await res.json();
    const data = json.data || {};

    if (json.success && data.success) {
      showToast(`Koneksi Silvex AI Sukses! Saldo: $${(data.balance || 0).toFixed(2)}`);

      if (modalBody) {
        modalTitle.textContent = '✅ Koneksi Silvex AI Berhasil';
        modalBody.innerHTML = `
          <div style="background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.3); border-radius: 10px; padding: 14px; margin-bottom: 16px;">
            <div style="font-weight: 700; color: #34d399; font-size: 0.95rem; margin-bottom: 4px;">● Status: TERHUBUNG (HTTP ${data.http_status})</div>
            <div style="color: #cbd5e1; font-size: 0.82rem;">${escapeHtml(data.message || 'API Key valid dan reseller Silvex AI merespons normal.')}</div>
          </div>

          <table style="width: 100%; border-collapse: collapse; font-size: 0.84rem;">
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Saldo Dompet Silvex</td>
              <td style="padding: 8px 0; font-weight: 800; font-family: var(--font-mono); font-size: 1.1rem; color: #a855f7; text-align: right;">$${(data.balance || 0).toFixed(2)} USD</td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Base URL</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #93c5fd; text-align: right;">${escapeHtml(data.base_url || 'https://silvex-ai.com/api/public/reseller')}</td>
            </tr>
            <tr>
              <td style="padding: 8px 0; color: var(--text-muted);">x-api-key Terpasang</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #cbd5e1; text-align: right;"><code>${escapeHtml(data.api_key_masked || '***')}</code></td>
            </tr>
          </table>

          ${data.raw_response ? `
            <div style="margin-top: 14px;">
              <div style="font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; font-weight: 700; margin-bottom: 4px;">Respon Mentah Upstream:</div>
              <pre style="background: rgba(0,0,0,0.4); border: 1px solid rgba(255,255,255,0.08); padding: 8px; border-radius: 6px; font-size: 0.75rem; color: #94a3b8; overflow-x: auto; max-height: 120px;">${escapeHtml(data.raw_response)}</pre>
            </div>
          ` : ''}
        `;
      }
    } else {
      showToast('Gagal terhubung ke Silvex AI.');

      if (modalBody) {
        modalTitle.textContent = '⚠️ Gagal Terhubung ke Silvex AI';
        modalBody.innerHTML = `
          <div style="background: rgba(239, 68, 68, 0.12); border: 1px solid rgba(239, 68, 68, 0.35); border-radius: 10px; padding: 14px; margin-bottom: 16px;">
            <div style="font-weight: 700; color: #f87171; font-size: 0.95rem; margin-bottom: 4px;">● Status: GAGAL (HTTP ${data.http_status || '-'})</div>
            <div style="color: #cbd5e1; font-size: 0.82rem;">${escapeHtml(data.message || json.error || 'Server Silvex AI menolak permintaan.')}</div>
          </div>

          <table style="width: 100%; border-collapse: collapse; font-size: 0.84rem;">
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.06);">
              <td style="padding: 8px 0; color: var(--text-muted);">Base URL</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #93c5fd; text-align: right;">${escapeHtml(data.base_url || 'https://silvex-ai.com/api/public/reseller')}</td>
            </tr>
            <tr>
              <td style="padding: 8px 0; color: var(--text-muted);">x-api-key Terpasang</td>
              <td style="padding: 8px 0; font-family: var(--font-mono); color: #cbd5e1; text-align: right;"><code>${escapeHtml(data.api_key_masked || '(kosong)')}</code></td>
            </tr>
          </table>

          <div style="margin-top: 14px; background: rgba(255,255,255,0.03); border-radius: 8px; padding: 10px; font-size: 0.78rem; color: #94a3b8;">
            💡 <strong>Panduan Solusi:</strong><br>
            1. Periksa file <code>.env</code> di VPS, pastikan <code>SILVEX_API_KEY</code> sudah terisi key reseller Silvex.<br>
            2. Jika baru mengisi atau mengubah <code>.env</code>, jalankan <code>sudo systemctl restart storeku</code> di VPS agar aplikasi memuat key baru.
          </div>

          ${data.raw_response ? `
            <div style="margin-top: 14px;">
              <div style="font-size: 0.72rem; color: var(--text-muted); text-transform: uppercase; font-weight: 700; margin-bottom: 4px;">Respon Upstream:</div>
              <pre style="background: rgba(0,0,0,0.4); border: 1px solid rgba(255,255,255,0.08); padding: 8px; border-radius: 6px; font-size: 0.75rem; color: #f87171; overflow-x: auto; max-height: 120px;">${escapeHtml(data.raw_response)}</pre>
            </div>
          ` : ''}
        `;
      }
    }
  } catch (err) {
    if (modalBody) {
      modalTitle.textContent = '❌ Terjadi Kesalahan Jaringan';
      modalBody.innerHTML = `<div style="color: #f87171;">Tidak dapat mengirim request ke backend: ${escapeHtml(err.message)}</div>`;
    }
  }
}

// -------------------------------------------------------------
// 6. ADMIN LAST LOGIN MONITORING ENGINE
// -------------------------------------------------------------
let adminLoginHistory = [];

async function loadLastLoginInfo() {
  try {
    const res = await adminFetch('/api/admin/last-login');
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      const cur = json.data.last_login || json.data.current_login;
      adminLoginHistory = json.data.history || [];

      const timeEl = document.getElementById('topbarLastLoginTime');
      const locEl = document.getElementById('topbarLastLoginLoc');

      if (cur) {
        if (timeEl) timeEl.textContent = formatLoginTime(cur.login_at);
        if (locEl) {
          const locText = cur.location || cur.ip_address || 'Online';
          locEl.textContent = `${locText} (${cur.ip_address || 'Aktif'})`;
          locEl.title = `IP: ${cur.ip_address} • Lokasi: ${cur.location || 'Indonesia'} • Perangkat: ${cur.device_info || '-'}`;
        }
      } else {
        if (timeEl) timeEl.textContent = 'Sesi Aktif';
        if (locEl) locEl.textContent = 'Online';
      }
    }
  } catch (err) {
    console.warn('Failed to load last login info:', err);
    const timeEl = document.getElementById('topbarLastLoginTime');
    const locEl = document.getElementById('topbarLastLoginLoc');
    if (timeEl && (timeEl.textContent === '-' || timeEl.textContent === 'Memuat...')) timeEl.textContent = 'Sesi Aktif';
    if (locEl && (locEl.textContent === '-' || locEl.textContent === 'Memuat...')) locEl.textContent = 'Online';
  }
}

function formatLoginTime(isoStr) {
  if (!isoStr) return 'Baru Saja';
  try {
    const d = new Date(isoStr);
    if (isNaN(d.getTime()) || d.getFullYear() < 2020) return 'Baru Saja';
    return d.toLocaleString('id-ID', {
      timeZone: 'Asia/Jakarta',
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }) + ' WIB';
  } catch (e) {
    return 'Baru Saja';
  }
}

async function openLoginHistoryModal() {
  const modal = document.getElementById('loginHistoryModal');
  const tbody = document.getElementById('loginHistoryTableBody');
  if (!modal) return;

  if (tbody && adminLoginHistory.length === 0) {
    tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 24px; color: var(--text-muted);">Memuat riwayat login...</td></tr>`;
  }

  modal.classList.add('open');

  try {
    await loadLastLoginInfo();
  } catch (e) {}

  if (tbody) {
    if (adminLoginHistory.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5" style="text-align: center; padding: 24px; color: var(--text-muted);">Sesi login aktif saat ini sedang berjalan. Riwayat bertambah setiap kali login.</td></tr>`;
    } else {
      tbody.innerHTML = adminLoginHistory.map(item => `
        <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
          <td style="padding: 10px 8px; font-family: var(--font-mono); color: #fff; font-weight: 600; white-space: nowrap;">
            ${formatLoginTime(item.login_at)}
          </td>
          <td style="padding: 10px 8px; font-family: var(--font-mono); color: #fbbf24;">
            ${escapeHtml(item.ip_address || '-')}
          </td>
          <td style="padding: 10px 8px; color: #93c5fd;">
            ${escapeHtml(item.location || 'Indonesia')}
          </td>
          <td style="padding: 10px 8px; color: #cbd5e1; font-size: 0.78rem;">
            ${escapeHtml(item.device_info || item.user_agent || 'Browser Web')}
          </td>
          <td style="padding: 10px 8px; color: #a855f7; font-weight: 600;">
            ${escapeHtml(item.username || 'admin')}
          </td>
        </tr>
      `).join('');
    }
  }
}

// -------------------------------------------------------------
// 7. SECURITY AUDIT & THREAT MONITORING ENGINE
// -------------------------------------------------------------
let allSecurityLogs = [];
let currentSecFilter = 'ALL';
let secCurrentPage = 1;
let secCurrentLimit = 50;
let secCurrentOrder = 'desc';
let secTotalCount = 0;

async function loadSecurityStats() {
  try {
    const res = await adminFetch('/api/admin/security/stats');
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      const s = json.data;
      const elSuccess = document.getElementById('secStatSuccess');
      if (elSuccess) elSuccess.textContent = s.total_success || 0;

      const elFailed = document.getElementById('secStatFailed');
      if (elFailed) elFailed.textContent = s.total_failed || 0;

      const elAll = document.getElementById('countAllLogs');
      if (elAll) elAll.textContent = (s.total_success || 0) + (s.total_failed || 0);

      const elSucc = document.getElementById('countSuccessLogs');
      if (elSucc) elSucc.textContent = s.total_success || 0;

      const elFail = document.getElementById('countFailedLogs');
      if (elFail) elFail.textContent = s.total_failed || 0;

      const elLastFailedIP = document.getElementById('secStatLastFailedIP');
      const elLastFailedMeta = document.getElementById('secStatLastFailedMeta');
      if (elLastFailedIP) {
        if (s.last_failed_ip) {
          elLastFailedIP.textContent = s.last_failed_ip;
          if (elLastFailedMeta) {
            elLastFailedMeta.innerHTML = `User: <strong>'${escapeHtml(s.last_failed_user || 'admin')}'</strong> • ${formatLoginTime(s.last_failed_at)}`;
          }
        } else {
          elLastFailedIP.textContent = 'Aman (0)';
          if (elLastFailedMeta) elLastFailedMeta.textContent = 'Tidak ada upaya gagal terdeteksi';
        }
      }

      const elCurIP = document.getElementById('secStatCurrentIP');
      const elCurMeta = document.getElementById('secStatCurrentMeta');
      if (elCurIP && s.current_session) {
        elCurIP.textContent = s.current_session.ip_address || '127.0.0.1';
        if (elCurMeta) {
          elCurMeta.textContent = `${s.current_session.location || 'Indonesia'} • ${s.current_session.device_info || '-'}`;
        }
      }
    }
  } catch (err) {
    console.warn('Failed to load security stats:', err);
  }
}

async function loadSecurityLogs() {
  const tbody = document.getElementById('secLogsTableBody');
  if (!tbody) return;

  try {
    const search = (document.getElementById('secSearchInput')?.value || '').trim();
    const offset = (secCurrentPage - 1) * secCurrentLimit;
    let url = `/api/admin/security/logs?status=${encodeURIComponent(currentSecFilter)}&limit=${secCurrentLimit}&offset=${offset}&order=${secCurrentOrder}`;
    if (search) url += `&search=${encodeURIComponent(search)}`;

    const res = await adminFetch(url);
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      allSecurityLogs = json.data.logs || [];
      secTotalCount = json.data.total !== undefined ? json.data.total : allSecurityLogs.length;
      renderSecurityTable();
      renderSecurityPagination();
    }
  } catch (err) {
    if (tbody) tbody.innerHTML = `<tr><td colspan="7" style="text-align: center; color: #f87171; padding: 2rem;">Gagal memuat log: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function renderSecurityPagination() {
  const infoEl = document.getElementById('secPaginationInfo');
  const prevBtn = document.getElementById('btnSecPrev');
  const nextBtn = document.getElementById('btnSecNext');
  const pageIndicator = document.getElementById('secPageIndicator');

  const totalPages = Math.max(1, Math.ceil(secTotalCount / secCurrentLimit));
  if (secCurrentPage > totalPages) {
    secCurrentPage = totalPages;
  }

  const start = secTotalCount === 0 ? 0 : (secCurrentPage - 1) * secCurrentLimit + 1;
  const end = Math.min(secCurrentPage * secCurrentLimit, secTotalCount);

  if (infoEl) {
    infoEl.textContent = `Menampilkan ${start} - ${end} dari ${secTotalCount} log`;
  }

  if (pageIndicator) {
    pageIndicator.textContent = `Halaman ${secCurrentPage} / ${totalPages}`;
  }

  if (prevBtn) {
    prevBtn.disabled = secCurrentPage <= 1;
    prevBtn.style.opacity = secCurrentPage <= 1 ? '0.4' : '1';
    prevBtn.style.cursor = secCurrentPage <= 1 ? 'not-allowed' : 'pointer';
  }

  if (nextBtn) {
    nextBtn.disabled = secCurrentPage >= totalPages;
    nextBtn.style.opacity = secCurrentPage >= totalPages ? '0.4' : '1';
    nextBtn.style.cursor = secCurrentPage >= totalPages ? 'not-allowed' : 'pointer';
  }
}

function renderSecurityTable() {
  const tbody = document.getElementById('secLogsTableBody');
  if (!tbody) return;

  if (allSecurityLogs.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="7" style="text-align: center; padding: 3rem; color: var(--text-muted);">
          Tidak ada riwayat login yang cocok dengan filter pencarian.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = allSecurityLogs.map(l => {
    const isSuccess = l.status === 'SUCCESS';
    const statusPill = isSuccess
      ? `<span class="badge-sec-success">● BERHASIL</span>`
      : `<span class="badge-sec-failed">✕ GAGAL</span>`;

    const ipClass = isSuccess ? 'ip-success-badge' : 'ip-failed-badge';
    const reasonText = l.failure_reason || (isSuccess ? 'Login berhasil' : 'Ditolak');

    let reasonHtml = `<span style="font-size: 0.8rem; color: ${isSuccess ? '#34d399' : '#f87171'}; font-weight: 500;">${escapeHtml(reasonText)}</span>`;

    // Highlight raw password attempted if present in failure_reason
    if (!isSuccess && reasonText.includes(' • Password dicoba: ')) {
      const parts = reasonText.split(' • Password dicoba: ');
      const mainReason = parts[0];
      const rawPassword = parts.slice(1).join(' • Password dicoba: ');
      let cleanPass = rawPassword;
      if (cleanPass.startsWith("'") && cleanPass.endsWith("'") && cleanPass.length >= 2) {
        cleanPass = cleanPass.substring(1, cleanPass.length - 1);
      }
      reasonHtml = `
        <div style="font-size: 0.8rem; color: #f87171; font-weight: 500; margin-bottom: 4px;">${escapeHtml(mainReason)}</div>
        <div style="display: inline-flex; align-items: center; gap: 4px; font-family: var(--font-mono); font-size: 0.76rem; color: #fbbf24; background: rgba(251, 191, 36, 0.1); border: 1px solid rgba(251, 191, 36, 0.25); padding: 2px 6px; border-radius: 4px; word-break: break-all;">
          <span style="opacity: 0.85;">🔑 Password dicoba:</span>
          <strong style="color: #fff; font-weight: 700;">${escapeHtml(cleanPass)}</strong>
        </div>
      `;
    }

    return `
      <tr style="${isSuccess ? '' : 'background: rgba(239, 68, 68, 0.04);'}">
        <td>
          <div style="font-weight: 700; color: #fff; font-family: var(--font-mono); font-size: 0.82rem;">${formatLoginTime(l.login_at)}</div>
        </td>
        <td>
          <span style="font-weight: 700; color: ${isSuccess ? '#a855f7' : '#f87171'}; font-family: var(--font-mono); font-size: 0.85rem;">${escapeHtml(l.username || 'admin')}</span>
        </td>
        <td>
          ${statusPill}
        </td>
        <td>
          <code class="${ipClass}" style="font-size: 0.85rem;">${escapeHtml(l.ip_address)}</code>
        </td>
        <td>
          <div style="color: #93c5fd; font-size: 0.84rem; font-weight: 500;">${escapeHtml(l.location || 'Indonesia')}</div>
        </td>
        <td>
          <div style="color: #cbd5e1; font-size: 0.78rem;">${escapeHtml(l.device_info || l.user_agent || '-')}</div>
        </td>
        <td>
          ${reasonHtml}
        </td>
      </tr>
    `;
  }).join('');
}

// ==========================================
// VOUCHER MANAGEMENT LOGIC
// ==========================================
let allAdminVouchers = [];

async function loadAdminVouchers() {
  const tbody = document.getElementById('vouchersTableBody');
  if (!tbody) return;

  try {
    const res = await adminFetch('/api/admin/vouchers');
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      allAdminVouchers = json.data;
      updateVoucherStats();
      renderAdminVouchersTable(allAdminVouchers);
    } else {
      tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: #f87171; padding: 2rem;">Gagal memuat voucher: ${escapeHtml(json.error || 'Server error')}</td></tr>`;
    }
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; color: #f87171; padding: 2rem;">Gagal menghubungi server admin.</td></tr>`;
  }
}

function updateVoucherStats() {
  const totalEl = document.getElementById('statTotalVouchers');
  const activeEl = document.getElementById('statActiveVouchers');
  const usedEl = document.getElementById('statUsedVouchers');

  if (totalEl) totalEl.textContent = allAdminVouchers.length;

  const now = new Date();
  const activeCount = allAdminVouchers.filter(v => {
    if (!v.is_active) return false;
    if (v.used_count >= v.max_usage) return false;
    if (v.expires_at && new Date(v.expires_at) < now) return false;
    return true;
  }).length;
  if (activeEl) activeEl.textContent = activeCount;

  const totalUsed = allAdminVouchers.reduce((sum, v) => sum + (v.used_count || 0), 0);
  if (usedEl) usedEl.textContent = totalUsed;
}

function renderAdminVouchersTable(vouchers) {
  const tbody = document.getElementById('vouchersTableBody');
  if (!tbody) return;

  if (!vouchers || vouchers.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="8" style="text-align: center; color: var(--text-muted); padding: 2.5rem;">
          Belum ada voucher yang dibuat. Silakan terbitkan voucher promo baru melalui formulir di atas.
        </td>
      </tr>
    `;
    return;
  }

  const now = new Date();

  tbody.innerHTML = vouchers.map(v => {
    const isExpired = v.expires_at && new Date(v.expires_at) < now;
    const isQuotaFull = v.used_count >= v.max_usage;

    let statusPill = '<span class="status-pill paid">● Aktif</span>';
    if (!v.is_active) {
      statusPill = '<span class="status-pill failed">● Nonaktif</span>';
    } else if (isQuotaFull) {
      statusPill = '<span class="status-pill warning" style="background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.35);">● Kuota Habis</span>';
    } else if (isExpired) {
      statusPill = '<span class="status-pill warning" style="background: rgba(239, 68, 68, 0.15); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.35);">● Kedaluwarsa</span>';
    }

    const usagePercent = Math.min(100, Math.round((v.used_count / v.max_usage) * 100));
    const hmacBadge = v.signature_valid
      ? '<span class="status-pill paid" style="font-size: 0.72rem; padding: 2px 8px;" title="Tanda tangan kriptografis HMAC terverifikasi sah">🔒 HMAC Sah</span>'
      : '<span class="status-pill failed" style="font-size: 0.72rem; padding: 2px 8px;" title="Peringatan: Tanda tangan HMAC tidak cocok!">⚠️ Tidak Sah</span>';

    const minSpendText = v.min_spend_idr > 0 ? formatIDR(v.min_spend_idr) : '<span style="color: var(--text-muted); font-size: 0.8rem;">Tanpa Minimum</span>';
    const expiresText = v.expires_at ? formatLoginTime(v.expires_at) : '<span style="color: var(--text-muted); font-size: 0.8rem;">Selamanya</span>';

    return `
      <tr>
        <td>
          <div style="display: flex; align-items: center; gap: 8px;">
            <code style="font-weight: 800; font-family: var(--font-mono); font-size: 0.92rem; color: #38bdf8; background: rgba(56, 189, 248, 0.08); padding: 3px 8px; border-radius: 6px; border: 1px solid rgba(56, 189, 248, 0.2);">
              ${escapeHtml(v.code)}
            </code>
            <button class="btn-action-mini" onclick="copyVoucherCode('${escapeHtml(v.code)}')" title="Salin Kode Voucher" style="padding: 2px 6px; font-size: 0.75rem;">
              📋 Salin
            </button>
          </div>
          ${v.note ? `<div style="font-size: 0.73rem; color: var(--text-muted); margin-top: 3px;">📝 ${escapeHtml(v.note)}</div>` : ''}
        </td>
        <td>
          <div style="font-family: var(--font-mono); font-weight: 800; color: #10b981; font-size: 0.95rem;">
            -${formatIDR(v.discount_idr)}
          </div>
        </td>
        <td>
          <div style="font-size: 0.82rem; font-weight: 700; color: #fff; margin-bottom: 3px;">
            ${v.used_count} / ${v.max_usage} kali
          </div>
          <div style="width: 100px; height: 6px; background: rgba(255,255,255,0.08); border-radius: 3px; overflow: hidden;">
            <div style="width: ${usagePercent}%; height: 100%; background: ${usagePercent >= 100 ? '#ef4444' : '#60a5fa'}; border-radius: 3px;"></div>
          </div>
        </td>
        <td>
          <div style="font-size: 0.85rem;">${minSpendText}</div>
        </td>
        <td>
          <div style="font-size: 0.82rem; color: ${isExpired ? '#f87171' : '#cbd5e1'};">${expiresText}</div>
        </td>
        <td>
          ${hmacBadge}
        </td>
        <td>
          ${statusPill}
        </td>
        <td style="text-align: right;">
          <div style="display: flex; gap: 6px; justify-content: flex-end;">
            <button class="btn-action-mini" onclick="toggleVoucherStatus(${v.id})" style="font-size: 0.75rem; color: ${v.is_active ? '#fbbf24' : '#10b981'};" title="${v.is_active ? 'Nonaktifkan voucher' : 'Aktifkan voucher'}">
              ${v.is_active ? '⏸ Matikan' : '▶ Aktifkan'}
            </button>
            <button class="btn-action-mini" onclick="deleteVoucher(${v.id})" style="font-size: 0.75rem; color: #f87171;" title="Hapus voucher">
              🗑️ Hapus
            </button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function filterVouchersTable() {
  const query = (document.getElementById('searchVoucherInput')?.value || '').trim().toLowerCase();
  if (!query) {
    renderAdminVouchersTable(allAdminVouchers);
    return;
  }
  const filtered = allAdminVouchers.filter(v => {
    return v.code.toLowerCase().includes(query) || (v.note && v.note.toLowerCase().includes(query));
  });
  renderAdminVouchersTable(filtered);
}

function generateRandomVoucherCode() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  let rand = '';
  for (let i = 0; i < 6; i++) {
    rand += chars.charAt(Math.floor(Math.random() * chars.length));
  }
  const code = `DM-${rand}`;
  const input = document.getElementById('vCode');
  if (input) {
    input.value = code;
    input.focus();
  }
  showToast(`Kode unik di-generate: ${code} 🎲`);
}

function copyVoucherCode(code) {
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(code).then(() => {
      showToast(`Kode ${code} disalin ke clipboard! 📋`);
    }).catch(() => {
      fallbackCopy(code);
    });
  } else {
    fallbackCopy(code);
  }
}

function fallbackCopy(text) {
  const ta = document.createElement('textarea');
  ta.value = text;
  document.body.appendChild(ta);
  ta.select();
  document.execCommand('copy');
  document.body.removeChild(ta);
  showToast(`Kode ${text} disalin! 📋`);
}

async function submitCreateVoucher() {
  const btn = document.getElementById('btnSubmitVoucher');
  const code = (document.getElementById('vCode')?.value || '').trim().toUpperCase();
  const discount = parseInt(document.getElementById('vDiscount')?.value, 10) || 0;
  const maxUsage = parseInt(document.getElementById('vMaxUsage')?.value, 10) || 1;
  const minSpend = parseInt(document.getElementById('vMinSpend')?.value, 10) || 0;
  const expiresAt = (document.getElementById('vExpiresAt')?.value || '').trim();
  const note = (document.getElementById('vNote')?.value || '').trim();
  const password = (document.getElementById('vAdminPassword')?.value || '').trim();

  if (!code) {
    alert('Kode voucher wajib diisi!');
    document.getElementById('vCode')?.focus();
    return;
  }
  if (discount < 500) {
    alert('Potongan diskon minimal Rp 500!');
    document.getElementById('vDiscount')?.focus();
    return;
  }
  if (!password) {
    alert('Password admin wajib diisi sebagai otorisasi keamanan penerbitan voucher!');
    document.getElementById('vAdminPassword')?.focus();
    return;
  }

  btn.disabled = true;
  btn.textContent = 'Menandatangani & Menyimpan...';

  try {
    const res = await adminFetch('/api/admin/vouchers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        code: code,
        discount_idr: discount,
        max_usage: maxUsage,
        min_spend_idr: minSpend,
        expires_at: expiresAt,
        note: note,
        admin_password: password
      })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast(json.message || 'Voucher berhasil diterbitkan! ✨');
      document.getElementById('createVoucherForm')?.reset();
      await loadAdminVouchers();
    } else {
      alert('Gagal menerbitkan voucher: ' + (json.error || 'Terjadi kesalahan'));
    }
  } catch (err) {
    alert('Gagal menghubungi server admin.');
  } finally {
    btn.disabled = false;
    btn.textContent = '+ Terbitkan & Tanda Tangani Voucher';
  }
}

async function toggleVoucherStatus(id) {
  try {
    const res = await adminFetch(`/api/admin/vouchers/${id}/toggle`, {
      method: 'POST'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast(json.message || 'Status voucher diperbarui! ✨');
      await loadAdminVouchers();
    } else {
      alert('Gagal mengubah status: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Gagal menghubungi server admin.');
  }
}

async function deleteVoucher(id, code) {
  let vCode = code;
  if (!vCode && Array.isArray(allAdminVouchers)) {
    const found = allAdminVouchers.find(v => v.id === id);
    if (found) vCode = found.code;
  }
  if (!vCode) vCode = `ID #${id}`;

  if (!confirm(`Yakin ingin menghapus permanen voucher "${vCode}"?\nVoucher yang sudah dihapus tidak dapat dipulihkan.`)) {
    return;
  }

  try {
    const res = await adminFetch(`/api/admin/vouchers/${id}`, {
      method: 'DELETE'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast(json.message || 'Voucher berhasil dihapus! 🗑️');
      await loadAdminVouchers();
    } else {
      alert('Gagal menghapus: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Gagal menghubungi server admin.');
  }
}

// -------------------------------------------------------------
// 9. SQLITE MAINTENANCE & CLEAN ENGINE
// -------------------------------------------------------------
async function loadDatabaseStats() {
  const badge = document.getElementById('dbSizeBadge');
  if (!badge) return;

  try {
    const res = await adminFetch('/api/admin/maintenance/stats');
    if (!res) return;
    const json = await res.json();
    if (json.success && json.data) {
      const d = json.data;
      badge.textContent = `💾 Ukuran: ${d.file_size_human}`;

      const elTotalOrders = document.getElementById('dbStatsTotalOrders');
      if (elTotalOrders) elTotalOrders.textContent = d.total_orders;

      const elExpiredOrders = document.getElementById('dbStatsExpiredOrders');
      if (elExpiredOrders) elExpiredOrders.textContent = d.expired_orders;

      const elOrdersWithKeys = document.getElementById('dbStatsOrdersWithKeys');
      if (elOrdersWithKeys) elOrdersWithKeys.textContent = d.orders_with_delivered_items;

      const elOldKeysCount = document.getElementById('dbStatsOldKeysCount');
      if (elOldKeysCount) elOldKeysCount.textContent = `${d.delivered_keys_older_3d} item`;

      const elAdminLogs = document.getElementById('dbStatsAdminLogs');
      if (elAdminLogs) elAdminLogs.textContent = d.total_admin_logs;

      const elFailedLogs = document.getElementById('dbStatsFailedLogs');
      if (elFailedLogs) elFailedLogs.textContent = d.failed_admin_logs;

      const elCachedProducts = document.getElementById('dbStatsCachedProducts');
      if (elCachedProducts) elCachedProducts.textContent = `${d.cached_products_count} cache`;

      const elVouchers = document.getElementById('dbStatsVouchers');
      if (elVouchers) elVouchers.textContent = d.total_vouchers;

      const elModalSize = document.getElementById('modalCurrentDbSize');
      if (elModalSize) elModalSize.textContent = `${d.file_size_human} (${Number(d.file_size_bytes || 0).toLocaleString('id-ID')} bytes)`;
    }
  } catch (err) {
    console.error('Failed to load database stats:', err);
  }
}

function openDbMaintenanceModal() {
  const modal = document.getElementById('dbMaintenanceModal');
  if (!modal) return;
  loadDatabaseStats();
  const pwdInput = document.getElementById('cleanAdminPassword');
  if (pwdInput) pwdInput.value = '';
  modal.classList.add('open');
}

function closeDbMaintenanceModal() {
  const modal = document.getElementById('dbMaintenanceModal');
  if (modal) modal.classList.remove('open');
}

function setupDbMaintenanceUI() {
  // Checkbox sub-option display toggles
  const ordersCheck = document.getElementById('cleanOrdersCheck');
  const ordersOptions = document.getElementById('cleanOrdersOptions');
  const ordersScope = document.getElementById('cleanOrdersScope');
  const ordersDays = document.getElementById('cleanOrdersDays');

  if (ordersCheck && ordersOptions) {
    ordersCheck.addEventListener('change', () => {
      ordersOptions.style.display = ordersCheck.checked ? 'flex' : 'none';
    });
  }

  if (ordersScope && ordersDays) {
    ordersScope.addEventListener('change', () => {
      ordersDays.style.display = ordersScope.value === 'older_than' ? 'inline-block' : 'none';
    });
  }

  const keysCheck = document.getElementById('cleanKeysCheck');
  const keysOptions = document.getElementById('cleanKeysOptions');
  if (keysCheck && keysOptions) {
    keysCheck.addEventListener('change', () => {
      keysOptions.style.display = keysCheck.checked ? 'block' : 'none';
    });
  }

  const logsCheck = document.getElementById('cleanLogsCheck');
  const logsOptions = document.getElementById('cleanLogsOptions');
  if (logsCheck && logsOptions) {
    logsCheck.addEventListener('change', () => {
      logsOptions.style.display = logsCheck.checked ? 'block' : 'none';
    });
  }

  const vouchersCheck = document.getElementById('cleanVouchersCheck');
  const vouchersOptions = document.getElementById('cleanVouchersOptions');
  if (vouchersCheck && vouchersOptions) {
    vouchersCheck.addEventListener('change', () => {
      vouchersOptions.style.display = vouchersCheck.checked ? 'block' : 'none';
    });
  }

  // Open & Close triggers
  document.getElementById('btnOpenDbMaintenance')?.addEventListener('click', openDbMaintenanceModal);
  document.getElementById('closeDbMaintenanceModalBtn')?.addEventListener('click', closeDbMaintenanceModal);
  document.getElementById('closeDbMaintenanceModalBtn2')?.addEventListener('click', closeDbMaintenanceModal);

  // Submit trigger
  document.getElementById('btnSubmitDbClean')?.addEventListener('click', submitDatabaseClean);
}

async function submitDatabaseClean() {
  const cleanOrders = document.getElementById('cleanOrdersCheck')?.checked || false;
  const cleanKeys = document.getElementById('cleanKeysCheck')?.checked || false;
  const cleanLogs = document.getElementById('cleanLogsCheck')?.checked || false;
  const cleanVouchers = document.getElementById('cleanVouchersCheck')?.checked || false;
  const cleanCached = document.getElementById('cleanCachedProductsCheck')?.checked || false;
  const vacuum = document.getElementById('vacuumDbCheck')?.checked || false;
  const password = (document.getElementById('cleanAdminPassword')?.value || '').trim();

  if (!cleanOrders && !cleanKeys && !cleanLogs && !cleanVouchers && !cleanCached && !vacuum) {
    alert('Silakan pilih setidaknya satu opsi pembersihan data atau optimasi SQLite VACUUM!');
    return;
  }

  if (!password) {
    alert('Password admin wajib diisi sebagai otorisasi keamanan tindakan pembersihan database!');
    document.getElementById('cleanAdminPassword')?.focus();
    return;
  }

  const ordersScope = document.getElementById('cleanOrdersScope')?.value || 'expired';
  const ordersDays = parseInt(document.getElementById('cleanOrdersDays')?.value || '3', 10);
  const keysDays = parseInt(document.getElementById('cleanKeysDays')?.value || '3', 10);
  const logsScope = document.getElementById('cleanLogsScope')?.value || 'older_than_7';
  const vouchersScope = document.getElementById('cleanVouchersScope')?.value || 'expired_or_used';

  let confirmMsg = 'Konfirmasi eksekusi pemeliharaan database SQLite?\n\nTindakan ini akan memproses opsi terpilih secara permanen.';
  if (cleanOrders && ordersScope === 'all') {
    confirmMsg = '⚠️ PERINGATAN KRITIS: Anda memilih opsi menghapus SELURUH riwayat pesanan toko!\n\nTindakan ini bersifat PERMANEN dan TIDAK DAPAT DIBATALKAN.\nApakah Anda benar-benar yakin ingin melanjutkan?';
  }

  if (!confirm(confirmMsg)) {
    return;
  }

  const btn = document.getElementById('btnSubmitDbClean');
  btn.disabled = true;
  btn.textContent = '⏳ Menjalankan Pembersihan & VACUUM...';

  const payload = {
    clean_orders: cleanOrders,
    orders_mode: ordersScope.toUpperCase(),
    orders_scope: ordersScope,
    orders_older_days: ordersDays,
    orders_older_than_days: ordersDays,
    clean_delivered_keys: cleanKeys,
    keys_older_days: keysDays,
    delivered_keys_older_than_days: keysDays,
    clean_login_logs: cleanLogs,
    logs_mode: logsScope.toUpperCase(),
    login_logs_scope: logsScope,
    clean_vouchers: cleanVouchers,
    vouchers_mode: vouchersScope.toUpperCase(),
    vouchers_scope: vouchersScope,
    clean_cached_products: cleanCached,
    run_vacuum: vacuum,
    vacuum: vacuum,
    admin_password: password
  };

  try {
    const res = await adminFetch('/api/admin/maintenance/clean', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (!res) return;
    const json = await res.json();

    if (json.success && json.data) {
      const d = json.data;
      const sizeBefore = d.size_before || d.database_size_before || '-';
      const sizeAfter = d.size_after || d.database_size_after || '-';
      const spaceFreed = d.space_freed || '-';
      const ordersDel = d.orders_deleted ?? 0;
      const keysDel = d.delivered_keys_cleared ?? d.keys_cleaned ?? 0;
      const logsDel = d.admin_logs_deleted ?? d.logs_deleted ?? 0;
      const vouchersDel = d.vouchers_deleted ?? 0;
      const cacheDel = d.cached_products_deleted ?? d.cached_cleaned ?? 0;
      const isVacuumed = d.vacuumed || d.vacuum_executed;
      const summaryMsg = `✅ Pembersihan Database Berhasil Dilakukan!\n\n` +
        `• Ukuran Awal: ${sizeBefore}\n` +
        `• Ukuran Akhir: ${sizeAfter}\n` +
        `• Ruang Dihemat: ${spaceFreed}\n` +
        `• Pesanan Dihapus: ${ordersDel}\n` +
        `• Serial Key Dikosongkan: ${keysDel}\n` +
        `• Log Login Dihapus: ${logsDel}\n` +
        `• Voucher Dihapus: ${vouchersDel}\n` +
        `• Cache Dikosongkan: ${cacheDel}\n` +
        `• SQLite VACUUM: ${isVacuumed ? 'Selesai' : 'Dilewati'}`;

      alert(summaryMsg);
      showToast('Database SQLite berhasil dibersihkan! 🧹');
      closeDbMaintenanceModal();
      await loadDatabaseStats();
      if (typeof loadStats === 'function') await loadStats();
      if (typeof loadOrders === 'function') await loadOrders();
    } else {
      alert('Gagal membersihkan database: ' + (json.error || 'Terjadi kesalahan sistem'));
    }
  } catch (err) {
    alert('Gagal menghubungi server admin: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = '⚠️ Mulai Eksekusi Pembersihan';
  }
}

// -------------------------------------------------------------
// 7. PRODUK MANDIRI / LOKAL (CUSTOM PRODUCTS) LOGIC
// -------------------------------------------------------------
let allLocalProducts = [];
let filteredLocalProducts = [];
let currentLpFilter = 'ALL';
let currentEditingLocalProductId = null;
let currentQuickKeysProductId = null;

async function loadLocalProducts() {
  const tbody = document.getElementById('localProductsTableBody');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="6" style="text-align:center; padding: 2.5rem; color: var(--text-muted);">Memuat daftar produk mandiri...</td></tr>`;

  try {
    const res = await adminFetch('/api/admin/custom-products');
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) {
      tbody.innerHTML = `<tr><td colspan="6" style="text-align:center; padding: 2.5rem; color: #f87171;">Gagal memuat produk mandiri: ${escapeHtml(json.error || 'Server error')}</td></tr>`;
      return;
    }

    allLocalProducts = json.data;
    updateLocalProductsStats();
    applyLocalProductFilter();
  } catch (err) {
    console.error('Failed to load local products:', err);
    tbody.innerHTML = `<tr><td colspan="6" style="text-align:center; padding: 2.5rem; color: #f87171;">Koneksi gagal: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function updateLocalProductsStats() {
  const total = allLocalProducts.length;
  const active = allLocalProducts.filter(p => p.is_active && !p.is_force_out_of_stock).length;
  let totalKeys = 0;
  let emptyCount = 0;

  allLocalProducts.forEach(p => {
    const kCount = (p.stock_keys || []).length;
    totalKeys += kCount;
    if (kCount === 0) emptyCount++;
  });

  if (document.getElementById('cntTotalLocalProds')) document.getElementById('cntTotalLocalProds').textContent = total;
  if (document.getElementById('cntActiveLocalProds')) document.getElementById('cntActiveLocalProds').textContent = active;
  if (document.getElementById('cntTotalStockKeys')) document.getElementById('cntTotalStockKeys').textContent = totalKeys;
  if (document.getElementById('cntEmptyStockKeys')) document.getElementById('cntEmptyStockKeys').textContent = emptyCount;
}

function applyLocalProductFilter() {
  const query = (document.getElementById('searchLocalProdInput')?.value || '').toLowerCase().trim();

  filteredLocalProducts = allLocalProducts.filter(p => {
    // Tab filter
    const keyCount = (p.stock_keys || []).length;
    if (currentLpFilter === 'ACTIVE' && (!p.is_active || p.is_force_out_of_stock)) return false;
    if (currentLpFilter === 'HAS_KEYS' && keyCount === 0) return false;
    if (currentLpFilter === 'EMPTY_KEYS' && keyCount > 0) return false;
    if (currentLpFilter === 'HIDDEN' && p.is_active) return false;

    // Search query
    if (query) {
      const name = (p.name || '').toLowerCase();
      const slug = (p.slug || '').toLowerCase();
      const type = (p.type || '').toLowerCase();
      if (!name.includes(query) && !slug.includes(query) && !type.includes(query)) return false;
    }

    return true;
  });

  renderLocalProductsTable();
}

function renderLocalProductsTable() {
  const tbody = document.getElementById('localProductsTableBody');
  if (!tbody) return;

  if (filteredLocalProducts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align:center; padding: 3rem; color: var(--text-muted);">Tidak ada produk mandiri yang sesuai filter atau pencarian.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredLocalProducts.map(p => {
    const keyCount = (p.stock_keys || []).length;
    const hasPromo = p.has_promo && p.promo_price_idr > 0;
    const defaultLogo = p.logo || '/icons/package.svg';

    return `
      <tr>
        <td>
          <div style="display: flex; align-items: center; gap: 12px;">
            <img src="${escapeHtml(defaultLogo)}" alt="${escapeHtml(p.name)}" 
                 style="width: 42px; height: 42px; border-radius: 8px; object-fit: cover; background: rgba(255,255,255,0.05); border: 1px solid var(--border-subtle);"
                 onerror="this.src='/icons/package.svg';">
            <div>
              <div style="font-weight: 700; color: #fff; font-size: 0.92rem; display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                <span>${escapeHtml(p.name)}</span>
                <span class="badge" style="background: rgba(99,102,241,0.15); color: #a5b4fc; font-size: 0.7rem; border: 1px solid rgba(99,102,241,0.3);">✨ Mandiri</span>
              </div>
              <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: 3px;">
                <code style="color: #94a3b8; font-size: 0.72rem;">/${escapeHtml(p.slug)}</code> • ${escapeHtml(p.type || 'Digital')}
              </div>
            </div>
          </div>
        </td>
        <td>
          <div style="font-size: 0.82rem; font-weight: 600; color: #cbd5e1;">${escapeHtml(p.region || 'Global')}</div>
          <div style="font-size: 0.75rem; color: #10b981; margin-top: 2px;">🛡️ ${escapeHtml(p.warranty || 'Garansi Toko')}</div>
        </td>
        <td>
          ${hasPromo ? `
            <div>
              <span style="font-weight: 700; color: #fbbf24; font-family: var(--font-mono); font-size: 0.95rem;">${formatIDR(p.promo_price_idr)}</span>
              <span class="badge" style="background: rgba(245,158,11,0.2); color: #fbbf24; font-size: 0.68rem; margin-left: 4px;">PROMO</span>
            </div>
            <div style="text-decoration: line-through; color: var(--text-muted); font-size: 0.75rem; font-family: var(--font-mono);">${formatIDR(p.price_idr)}</div>
          ` : `
            <div style="font-weight: 700; color: #10b981; font-family: var(--font-mono); font-size: 0.95rem;">${formatIDR(p.price_idr)}</div>
          `}
        </td>
        <td>
          <div style="display: flex; align-items: center; gap: 6px;">
            ${keyCount > 0 ? `
              <span class="badge badge-stock" style="font-size: 0.75rem; font-weight: 700;">
                ● ${keyCount}/10 Key
              </span>
            ` : `
              <span class="badge" style="background: rgba(59,130,246,0.15); color: #60a5fa; border: 1px solid rgba(59,130,246,0.3); font-size: 0.75rem; font-weight: 600;">
                ○ 0 Key (Auto Kontak CS)
              </span>
            `}
          </div>
          <div style="margin-top: 4px;">
            <button class="btn-action-mini" style="font-size: 0.7rem; padding: 2px 8px; color: #a5b4fc; background: rgba(99,102,241,0.12); border-color: rgba(99,102,241,0.3);" onclick="openQuickKeysModal(${p.id})">
              🔑 Kelola Key (${keyCount})
            </button>
          </div>
        </td>
        <td>
          ${p.is_force_out_of_stock ? `
            <span class="badge badge-stock empty" style="font-size: 0.72rem;">⚡ Stok Habis</span>
          ` : p.is_active ? `
            <span class="badge badge-stock" style="font-size: 0.72rem;">● Aktif</span>
          ` : `
            <span class="badge badge-region" style="font-size: 0.72rem; color: #94a3b8;">○ Nonaktif</span>
          `}
        </td>
        <td style="text-align: right;">
          <div style="display: flex; gap: 4px; justify-content: flex-end; flex-wrap: wrap;">
            <button class="btn-action-mini" onclick="editLocalProduct(${p.id})">Edit</button>
            <button class="btn-action-mini" style="color: #f87171; border-color: rgba(239,68,68,0.3); background: rgba(239,68,68,0.1);" onclick="deleteLocalProduct(${p.id})">Hapus</button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function openCreateLocalProductModal() {
  currentEditingLocalProductId = null;
  document.getElementById('localProdModalTitle').textContent = '✨ Tambah Produk Mandiri Baru';
  document.getElementById('lpEditId').value = '';
  document.getElementById('lpName').value = '';
  document.getElementById('lpSlug').value = '';
  document.getElementById('lpType').value = 'Digital';
  document.getElementById('lpLogo').value = '';
  document.getElementById('lpRegion').value = 'Global';
  document.getElementById('lpWarranty').value = 'Full Garansi';
  document.getElementById('lpPriceIDR').value = '';
  document.getElementById('lpRefPriceIDR').value = '';
  document.getElementById('lpHasPromo').checked = false;
  document.getElementById('lpPromoOptions').style.display = 'none';
  document.getElementById('lpPromoPriceIDR').value = '';
  document.getElementById('lpStockKeys').value = '';
  updateLocalKeysCounter();
  document.getElementById('lpDescription').value = '';
  document.getElementById('lpTerms').value = 'Membeli berarti menyetujui seluruh ketentuan garansi.';
  document.getElementById('lpIsActive').checked = true;
  document.getElementById('lpIsForceOut').checked = false;

  document.getElementById('localProductModal')?.classList.add('open');
}

function editLocalProduct(id) {
  const p = allLocalProducts.find(item => item.id === id);
  if (!p) return;

  currentEditingLocalProductId = id;
  document.getElementById('localProdModalTitle').textContent = '✏️ Edit Produk Mandiri: ' + p.name;
  document.getElementById('lpEditId').value = p.id;
  document.getElementById('lpName').value = p.name || '';
  document.getElementById('lpSlug').value = p.slug || '';
  document.getElementById('lpType').value = p.type || '';
  document.getElementById('lpLogo').value = p.logo || '';
  document.getElementById('lpRegion').value = p.region || 'Global';
  document.getElementById('lpWarranty').value = p.warranty || '';
  document.getElementById('lpPriceIDR').value = p.price_idr || '';
  document.getElementById('lpRefPriceIDR').value = '';
  document.getElementById('lpHasPromo').checked = !!p.has_promo;
  document.getElementById('lpPromoOptions').style.display = p.has_promo ? 'block' : 'none';
  document.getElementById('lpPromoPriceIDR').value = p.promo_price_idr || '';
  document.getElementById('lpStockKeys').value = (p.stock_keys || []).join('\n');
  updateLocalKeysCounter();
  document.getElementById('lpDescription').value = p.description || '';
  document.getElementById('lpTerms').value = p.terms || '';
  document.getElementById('lpIsActive').checked = p.is_active;
  document.getElementById('lpIsForceOut').checked = p.is_force_out_of_stock;

  document.getElementById('localProductModal')?.classList.add('open');
}

function closeLocalProductModal() {
  document.getElementById('localProductModal')?.classList.remove('open');
  currentEditingLocalProductId = null;
}

function updateLocalKeysCounter() {
  const val = document.getElementById('lpStockKeys')?.value || '';
  const lines = val.split('\n').map(s => s.trim()).filter(Boolean);
  const counterEl = document.getElementById('lpKeyCounter');
  if (counterEl) {
    counterEl.textContent = `${lines.length} / 10 key terisi`;
    if (lines.length > 10) {
      counterEl.style.color = '#f87171';
      counterEl.textContent = `${lines.length} / 10 key (Maksimal 10! Baris lebih akan dipotong)`;
    } else {
      counterEl.style.color = '#818cf8';
    }
  }
}

async function saveLocalProduct() {
  const btn = document.getElementById('btnSaveLocalProd');
  const name = document.getElementById('lpName')?.value.trim();
  const priceIDR = parseFloat(document.getElementById('lpPriceIDR')?.value);

  if (!name) {
    alert('Nama produk wajib diisi.');
    document.getElementById('lpName')?.focus();
    return;
  }
  if (isNaN(priceIDR) || priceIDR < 0) {
    alert('Harga produk wajib diisi dengan angka valid.');
    document.getElementById('lpPriceIDR')?.focus();
    return;
  }

  const rawKeys = (document.getElementById('lpStockKeys')?.value || '')
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean);

  if (rawKeys.length > 10) {
    if (!confirm(`Anda memasukkan ${rawKeys.length} serial key, sedangkan batas maksimal adalah 10 key. Lanjutkan dengan memotong 10 key pertama?`)) {
      return;
    }
  }

  const stockKeys = rawKeys.slice(0, 10);
  const hasPromo = document.getElementById('lpHasPromo')?.checked || false;
  let promoPrice = parseFloat(document.getElementById('lpPromoPriceIDR')?.value) || 0;
  if (!hasPromo) promoPrice = 0;

  const payload = {
    name: name,
    slug: document.getElementById('lpSlug')?.value.trim(),
    type: document.getElementById('lpType')?.value.trim() || 'Digital',
    logo: document.getElementById('lpLogo')?.value.trim(),
    region: document.getElementById('lpRegion')?.value.trim() || 'Global',
    warranty: document.getElementById('lpWarranty')?.value.trim() || 'Full Garansi',
    description: document.getElementById('lpDescription')?.value.trim(),
    terms: document.getElementById('lpTerms')?.value.trim(),
    price_idr: priceIDR,
    promo_price_idr: promoPrice,
    has_promo: hasPromo,
    is_active: document.getElementById('lpIsActive')?.checked ?? true,
    is_force_out_of_stock: document.getElementById('lpIsForceOut')?.checked ?? false,
    stock_keys: stockKeys
  };

  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  try {
    const isEdit = !!currentEditingLocalProductId;
    const url = isEdit ? `/api/admin/custom-products/${currentEditingLocalProductId}` : '/api/admin/custom-products';
    const method = isEdit ? 'PUT' : 'POST';

    const res = await adminFetch(url, {
      method: method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });

    if (!res) return;
    const json = await res.json();

    if (json.success) {
      showToast(isEdit ? 'Produk mandiri berhasil diperbarui! ✨' : 'Produk mandiri baru berhasil ditambahkan! 🚀');
      closeLocalProductModal();
      await loadLocalProducts();
    } else {
      alert('Gagal menyimpan produk: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Simpan Produk Mandiri';
  }
}

async function deleteLocalProduct(id, name) {
  const prod = allLocalProducts.find(item => item.id === id);
  const prodName = name || (prod ? prod.name : `ID #${id}`);
  if (!confirm(`Yakin ingin menghapus produk mandiri "${prodName}" secara permanen dari database? Seluruh sisa key untuk produk ini juga akan terhapus.`)) {
    return;
  }

  try {
    const res = await adminFetch(`/api/admin/custom-products/${id}`, {
      method: 'DELETE'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Produk mandiri berhasil dihapus.');
      await loadLocalProducts();
    } else {
      alert('Gagal menghapus produk: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  }
}

function openQuickKeysModal(productId) {
  const p = allLocalProducts.find(item => item.id === productId);
  if (!p) return;

  currentQuickKeysProductId = productId;
  document.getElementById('quickKeysModalTitle').textContent = `🔑 Kelola Key: ${p.name}`;
  document.getElementById('quickKeysModalDesc').textContent = `Masukkan serial key / lisensi (maksimal 10). Saat ini ada ${(p.stock_keys || []).length} key tersimpan.`;
  document.getElementById('quickKeysTextarea').value = (p.stock_keys || []).join('\n');
  updateQuickKeysCounter();

  document.getElementById('quickKeysModal')?.classList.add('open');
}

function closeQuickKeysModal() {
  document.getElementById('quickKeysModal')?.classList.remove('open');
  currentQuickKeysProductId = null;
}

function updateQuickKeysCounter() {
  const lines = (document.getElementById('quickKeysTextarea')?.value || '')
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean);
  const counterEl = document.getElementById('quickKeysCounter');
  if (counterEl) {
    counterEl.textContent = `${lines.length} / 10 key`;
    if (lines.length > 10) {
      counterEl.style.color = '#f87171';
      counterEl.textContent = `${lines.length} / 10 key (Maksimal 10! Lebih akan dipotong)`;
    } else {
      counterEl.style.color = '#818cf8';
    }
  }
}

async function saveQuickKeys() {
  if (!currentQuickKeysProductId) return;
  const p = allLocalProducts.find(item => item.id === currentQuickKeysProductId);
  if (!p) return;

  const btn = document.getElementById('btnSaveQuickKeys');
  const rawKeys = (document.getElementById('quickKeysTextarea')?.value || '')
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean);

  if (rawKeys.length > 10) {
    if (!confirm(`Anda memasukkan ${rawKeys.length} key, melebihi kuota 10 key. Potong menjadi 10 key pertama?`)) return;
  }

  const stockKeys = rawKeys.slice(0, 10);

  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  const payload = {
    name: p.name,
    slug: p.slug,
    type: p.type,
    logo: p.logo,
    region: p.region,
    warranty: p.warranty,
    description: p.description,
    terms: p.terms,
    price_idr: p.price_idr,
    promo_price_idr: p.promo_price_idr,
    has_promo: p.has_promo,
    is_active: p.is_active,
    is_force_out_of_stock: p.is_force_out_of_stock,
    stock_keys: stockKeys
  };

  try {
    const res = await adminFetch(`/api/admin/custom-products/${currentQuickKeysProductId}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast(`Stok key untuk ${p.name} berhasil diperbarui (${stockKeys.length} key) ✨`);
      closeQuickKeysModal();
      await loadLocalProducts();
    } else {
      alert('Gagal menyimpan key: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Simpan Perubahan Key';
  }
}

function setupLocalProductsUI() {
  document.getElementById('btnOpenCreateLocalProd')?.addEventListener('click', openCreateLocalProductModal);
  document.getElementById('closeLocalProdModalBtn')?.addEventListener('click', closeLocalProductModal);
  document.getElementById('cancelLocalProdBtn')?.addEventListener('click', closeLocalProductModal);
  document.getElementById('btnSaveLocalProd')?.addEventListener('click', saveLocalProduct);

  document.getElementById('lpHasPromo')?.addEventListener('change', (e) => {
    const opt = document.getElementById('lpPromoOptions');
    if (opt) opt.style.display = e.target.checked ? 'block' : 'none';
  });

  document.getElementById('lpStockKeys')?.addEventListener('input', updateLocalKeysCounter);

  // Filter tag buttons
  document.querySelectorAll('[data-lpfilter]').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('[data-lpfilter]').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      currentLpFilter = e.target.dataset.lpfilter;
      applyLocalProductFilter();
    });
  });

  // Search input
  document.getElementById('searchLocalProdInput')?.addEventListener('input', applyLocalProductFilter);

  // Quick keys modal handlers
  document.getElementById('closeQuickKeysModalBtn')?.addEventListener('click', closeQuickKeysModal);
  document.getElementById('cancelQuickKeysBtn')?.addEventListener('click', closeQuickKeysModal);
  document.getElementById('quickKeysTextarea')?.addEventListener('input', updateQuickKeysCounter);
  document.getElementById('btnSaveQuickKeys')?.addEventListener('click', saveQuickKeys);

  // Close modals on backdrop click
  document.getElementById('localProductModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'localProductModal') closeLocalProductModal();
  });
  document.getElementById('quickKeysModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'quickKeysModal') closeQuickKeysModal();
  });
}

// -------------------------------------------------------------
// 8. MANUAL DELIVER KEYS MODAL LOGIC (UNTUK ORDER)
// -------------------------------------------------------------
let currentManualDeliverInvoice = '';

function openManualDeliverModal(invoice) {
  currentManualDeliverInvoice = invoice;
  const invEl = document.getElementById('mdInvoiceText');
  if (invEl) invEl.textContent = invoice;
  const inputEl = document.getElementById('mdKeysInput');
  if (inputEl) inputEl.value = '';

  document.getElementById('manualDeliverModal')?.classList.add('open');
}

function closeManualDeliverModal() {
  document.getElementById('manualDeliverModal')?.classList.remove('open');
  currentManualDeliverInvoice = '';
}

async function submitManualDeliver() {
  if (!currentManualDeliverInvoice) return;
  const keys = (document.getElementById('mdKeysInput')?.value || '')
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean);

  if (keys.length === 0) {
    alert('Harap masukkan minimal 1 lisensi / serial key untuk dikirimkan ke pembeli.');
    document.getElementById('mdKeysInput')?.focus();
    return;
  }

  const btn = document.getElementById('btnSubmitManualDeliver');
  btn.disabled = true;
  btn.textContent = 'Mengirimkan Lisensi...';

  try {
    const res = await adminFetch(`/api/admin/orders/${encodeURIComponent(currentManualDeliverInvoice)}/manual-deliver-keys`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ keys: keys })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Lisensi berhasil dikirim dan pesanan diselesaikan! 🎉');
      closeManualDeliverModal();
      document.getElementById('orderDetailModal')?.classList.remove('open');
      if (typeof loadOrders === 'function') await loadOrders();
    } else {
      alert('Gagal mengirimkan lisensi: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Kirim Lisensi Sekarang';
  }
}

function setupManualDeliverUI() {
  document.getElementById('closeManualDeliverModalBtn')?.addEventListener('click', closeManualDeliverModal);
  document.getElementById('cancelManualDeliverBtn')?.addEventListener('click', closeManualDeliverModal);
  document.getElementById('btnSubmitManualDeliver')?.addEventListener('click', submitManualDeliver);
  document.getElementById('manualDeliverModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'manualDeliverModal') closeManualDeliverModal();
  });
}

// -------------------------------------------------------------
// 9. REVIEWS & ETALASE MANAGEMENT ENGINE (/admin/reviews)
// -------------------------------------------------------------
let allEtalaseProducts = [];
let filteredEtalaseProducts = [];
let allAdminReviews = [];
let filteredAdminReviews = [];

async function loadEtalaseProducts() {
  const tbody = document.getElementById('etalaseTableBody');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 2.5rem; color: var(--text-muted);">Memuat produk etalase & lokal...</td></tr>`;

  try {
    // 1. Fetch from unified etalase endpoint
    const res = await adminFetch('/api/admin/etalase-products');
    if (!res) return;

    let products = [];
    if (res.ok) {
      const json = await res.json();
      if (json.success && Array.isArray(json.data)) {
        products = json.data;
      }
    }

    // 2. Self-healing fallback: if unified endpoint returned empty or failed, fetch separately
    if (!products || products.length === 0) {
      try {
        const [pRes, lpRes] = await Promise.all([
          adminFetch('/api/admin/products').catch(() => null),
          adminFetch('/api/admin/custom-products').catch(() => null)
        ]);

        if (lpRes && lpRes.ok) {
          const lpJson = await lpRes.json();
          if (lpJson.success && Array.isArray(lpJson.data)) {
            lpJson.data.forEach(lp => {
              products.push({
                slug: lp.slug,
                name: lp.name,
                original_name: lp.name,
                type: lp.type,
                logo: lp.logo,
                region: lp.region,
                is_active: lp.is_active,
                display_order: lp.display_order || 0,
                sales_count: lp.sales_count || 10,
                source: 'LOCAL'
              });
            });
          }
        }

        if (pRes && pRes.ok) {
          const pJson = await pRes.json();
          if (pJson.success && Array.isArray(pJson.data)) {
            pJson.data.forEach(p => {
              products.push({
                slug: p.slug,
                name: p.name || p.original_name,
                original_name: p.original_name,
                type: p.type || p.original_type,
                logo: p.logo,
                region: p.region,
                is_active: p.is_active,
                display_order: p.display_order || 0,
                sales_count: p.sales_count || 10,
                source: 'DIGITALCORE'
              });
            });
          }
        }
      } catch (fallbackErr) {
        console.warn('Fallback catalog fetch error:', fallbackErr);
      }
    }

    allEtalaseProducts = products;

    // Update Product Select options in Create Review Modal & Review Filter
    populateProductSelectOptions(allEtalaseProducts);

    // Update stats
    const prioritizedCount = allEtalaseProducts.filter(p => p.display_order > 0).length;
    const prioEl = document.getElementById('cntPrioritizedProds');
    if (prioEl) prioEl.textContent = prioritizedCount;

    applyEtalaseFilter();

  } catch (err) {
    console.error('Failed to load etalase products:', err);
    tbody.innerHTML = `
      <tr>
        <td colspan="6" style="text-align: center; padding: 2.5rem; color: #f87171;">
          <div>Gagal memuat produk: ${escapeHtml(err.message)}</div>
          <button class="btn-nav" onclick="loadEtalaseProducts()" style="margin-top: 10px; font-size: 0.8rem;">🔄 Coba Lagi</button>
        </td>
      </tr>
    `;
  }
}

function populateProductSelectOptions(products) {
  const crSelect = document.getElementById('crProductSlug');
  const filterSelect = document.getElementById('filterReviewProductSelect');

  const currentCrVal = crSelect ? crSelect.value : '';
  const currentFilterVal = filterSelect ? filterSelect.value : '';

  if (crSelect) {
    crSelect.innerHTML = `<option value="">-- Pilih Produk --</option>` +
      products.map(p => {
        let srcLabel = 'DigitalCore';
        if (p.source === 'LOCAL') srcLabel = 'Lokal';
        else if (p.source === 'SILVEX') srcLabel = 'Silvex AI';
        return `<option value="${escapeHtml(p.slug)}">${escapeHtml(p.name)} (${srcLabel})</option>`;
      }).join('');
    if (currentCrVal) crSelect.value = currentCrVal;
  }

  if (filterSelect) {
    filterSelect.innerHTML = `<option value="">Semua Produk</option>` +
      products.map(p => `<option value="${escapeHtml(p.slug)}">${escapeHtml(p.name)}</option>`).join('');
    if (currentFilterVal) filterSelect.value = currentFilterVal;
  }
}

function applyEtalaseFilter() {
  const query = (document.getElementById('searchEtalaseInput')?.value || '').trim().toLowerCase();

  filteredEtalaseProducts = allEtalaseProducts.filter(p => {
    return (p.name || '').toLowerCase().includes(query) ||
           (p.slug || '').toLowerCase().includes(query) ||
           (p.type || '').toLowerCase().includes(query);
  });

  // Sort: display_order > 0 ascending first, then display_order == 0, then by name
  filteredEtalaseProducts.sort((a, b) => {
    const aOrder = a.display_order || 0;
    const bOrder = b.display_order || 0;
    if (aOrder > 0 && bOrder > 0) return aOrder - bOrder;
    if (aOrder > 0 && bOrder === 0) return -1;
    if (aOrder === 0 && bOrder > 0) return 1;
    return (a.name || '').localeCompare(b.name || '');
  });

  renderEtalaseTable();
}

function renderEtalaseTable() {
  const tbody = document.getElementById('etalaseTableBody');
  const countText = document.getElementById('etalaseProdCountText');
  if (!tbody) return;

  if (countText) {
    countText.textContent = `Menampilkan ${filteredEtalaseProducts.length} dari ${allEtalaseProducts.length} produk`;
  }

  if (filteredEtalaseProducts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 2.5rem; color: var(--text-muted);">Tidak ada produk yang cocok dengan pencarian.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredEtalaseProducts.map(p => {
    let sourceBadge = `<span class="badge" style="background: rgba(99, 102, 241, 0.15); color: #a5b4fc; border: 1px solid rgba(99, 102, 241, 0.3);">🌐 DigitalCore</span>`;
    if (p.source === 'LOCAL') {
      sourceBadge = `<span class="badge" style="background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.3);">✨ Lokal Mandiri</span>`;
    } else if (p.source === 'SILVEX') {
      sourceBadge = `<span class="badge" style="background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3);">🤖 Silvex AI</span>`;
    }

    const statusBadge = p.is_active
      ? `<span class="badge badge-stock">● Aktif</span>`
      : `<span class="badge badge-stock empty">● Nonaktif</span>`;

    const logoSrc = p.logo || 'https://via.placeholder.com/38?text=DC';
    const isPrio = (p.display_order || 0) > 0;
    const currentSales = p.sales_count !== undefined ? p.sales_count : 10;

    return `
      <tr data-slug="${escapeHtml(p.slug)}">
        <td>
          <div style="display: flex; align-items: center; gap: 10px;">
            <img src="${logoSrc}" alt="" style="width: 38px; height: 38px; border-radius: 8px; object-fit: cover; border: 1px solid var(--border-subtle);" onerror="this.src='https://via.placeholder.com/38?text=DC'">
            <div style="min-width: 0;">
              <div style="font-weight: 700; color: #fff; font-size: 0.9rem;">${escapeHtml(p.name)}</div>
              <div style="font-size: 0.75rem; color: var(--text-muted); font-family: var(--font-mono);">${escapeHtml(p.slug)}</div>
            </div>
          </div>
        </td>
        <td>
          <div>${sourceBadge}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted); margin-top: 3px;">${escapeHtml(p.type || 'Digital')}</div>
        </td>
        <td>${statusBadge}</td>
        <td style="text-align: center;">
          <input type="number" min="0" class="stat-input-num input-display-order ${isPrio ? 'highlight-gold' : ''}" value="${p.display_order || 0}">
          ${isPrio ? `<div style="font-size: 0.7rem; color: #fbbf24; font-weight: 600; margin-top: 2px;">★ Prioritas #${p.display_order}</div>` : `<div style="font-size: 0.7rem; color: var(--text-muted); margin-top: 2px;">Default (0)</div>`}
        </td>
        <td style="text-align: center;">
          <input type="number" min="0" class="stat-input-num input-sales-count" value="${currentSales}">
          <div style="font-size: 0.7rem; color: var(--text-muted); margin-top: 2px;">terjual</div>
        </td>
        <td style="text-align: center;">
          <button type="button" class="btn-buy btn-save-etalase" data-slug="${escapeHtml(p.slug)}" style="padding: 0.4rem 0.85rem; font-size: 0.8rem;">
            💾 Simpan
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

async function saveProductEtalaseStats(slug, rowEl, btn) {
  if (!slug || !rowEl) {
    alert('Elemen baris produk tidak ditemukan');
    return;
  }

  const orderEl = rowEl.querySelector('.input-display-order');
  const salesEl = rowEl.querySelector('.input-sales-count');

  if (!orderEl || !salesEl) {
    alert('Input data etalase tidak ditemukan');
    return;
  }

  const displayOrder = parseInt(orderEl.value, 10) || 0;
  const rawSales = parseInt(salesEl.value, 10);
  const salesCount = isNaN(rawSales) || rawSales < 0 ? 0 : rawSales;

  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Menyimpan...';
  }

  try {
    const res = await adminFetch(`/api/admin/products/${encodeURIComponent(slug)}/display-stats`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        display_order: displayOrder,
        sales_count: salesCount
      })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Urutan etalase & terjual disimpan! 💾');

      // Update in memory
      const item = allEtalaseProducts.find(p => p.slug === slug);
      if (item) {
        item.display_order = displayOrder;
        item.sales_count = salesCount;
      }

      const prioCount = allEtalaseProducts.filter(p => p.display_order > 0).length;
      const prioEl = document.getElementById('cntPrioritizedProds');
      if (prioEl) prioEl.textContent = prioCount;

      applyEtalaseFilter();
    } else {
      alert('Gagal menyimpan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = '💾 Simpan';
    }
  }
}

async function loadAdminReviews() {
  const tbody = document.getElementById('reviewsTableBody');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 2.5rem; color: var(--text-muted);">Memuat data ulasan...</td></tr>`;

  try {
    const res = await adminFetch('/api/admin/reviews?limit=200');
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) {
      tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 2.5rem; color: #f87171;">Gagal memuat ulasan: ${json.error || 'Server error'}</td></tr>`;
      return;
    }

    allAdminReviews = json.data;

    // Update stats
    const total = allAdminReviews.length;
    const visible = allAdminReviews.filter(r => r.is_visible).length;

    const cntTotalEl = document.getElementById('cntTotalReviews');
    if (cntTotalEl) cntTotalEl.textContent = total;

    const cntVisEl = document.getElementById('cntVisibleReviews');
    if (cntVisEl) cntVisEl.textContent = visible;

    applyReviewFilter();

  } catch (err) {
    console.error('Failed to load reviews:', err);
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 2.5rem; color: #f87171;">Error: ${err.message}</td></tr>`;
  }
}

function applyReviewFilter() {
  const query = (document.getElementById('searchReviewInput')?.value || '').trim().toLowerCase();
  const slugFilter = document.getElementById('filterReviewProductSelect')?.value || '';

  filteredAdminReviews = allAdminReviews.filter(r => {
    if (slugFilter && r.product_slug !== slugFilter) return false;
    if (!query) return true;
    return (r.author_name || '').toLowerCase().includes(query) ||
           (r.review_text || '').toLowerCase().includes(query) ||
           (r.product_slug || '').toLowerCase().includes(query) ||
           (r.order_invoice || '').toLowerCase().includes(query);
  });

  renderReviewsTable();
}

function renderReviewsTable() {
  const tbody = document.getElementById('reviewsTableBody');
  const countText = document.getElementById('reviewsCountText');
  if (!tbody) return;

  if (countText) {
    countText.textContent = `Menampilkan ${filteredAdminReviews.length} dari ${allAdminReviews.length} ulasan`;
  }

  if (filteredAdminReviews.length === 0) {
    tbody.innerHTML = `<tr><td colspan="8" style="text-align: center; padding: 2.5rem; color: var(--text-muted);">Tidak ada ulasan ditemukan.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredAdminReviews.map(r => {
    const isVisible = !!r.is_visible;
    const statusBadge = isVisible
      ? `<span class="badge badge-stock">● Tampil</span>`
      : `<span class="badge badge-stock empty">● Disembunyikan</span>`;

    const isVerified = !!r.order_invoice;
    const verifiedBadge = isVerified
      ? `<span class="verified-buyer-badge" style="font-size: 0.65rem;" title="Invoice: ${escapeHtml(r.order_invoice)}">✓ Pembeli Lunas</span>`
      : `<span class="badge" style="background: rgba(255,255,255,0.06); color: var(--text-muted); font-size: 0.65rem;">Manual Admin</span>`;

    let starsStr = '';
    for (let i = 1; i <= 5; i++) {
      starsStr += i <= r.rating ? '★' : '☆';
    }

    return `
      <tr>
        <td style="font-family: var(--font-mono); color: var(--text-muted); font-size: 0.8rem;">#${r.id}</td>
        <td>
          <div style="font-weight: 700; color: #fff; font-size: 0.88rem;">${escapeHtml(r.product_slug)}</div>
        </td>
        <td>
          <div style="font-weight: 600; color: #fff; font-size: 0.85rem;">${escapeHtml(r.author_name || 'Anonim')}</div>
          <div style="margin-top: 3px;">${verifiedBadge}</div>
        </td>
        <td>
          <div style="color: #fbbf24; font-size: 0.95rem; letter-spacing: 1px;">${starsStr}</div>
          <div style="font-size: 0.72rem; color: var(--text-muted); font-family: var(--font-mono);">${r.rating}.0 / 5.0</div>
        </td>
        <td>
          <div style="font-size: 0.82rem; color: #cbd5e1; max-width: 280px; line-height: 1.45; max-height: 80px; overflow-y: auto;">
            ${escapeHtml(r.review_text || '(Tidak ada teks ulasan)')}
          </div>
        </td>
        <td>${statusBadge}</td>
        <td style="font-size: 0.78rem; color: var(--text-muted);">${formatDate(r.created_at)}</td>
        <td style="text-align: center;">
          <div style="display: flex; gap: 6px; justify-content: center;">
            <button type="button" class="btn-table-action btn-edit-review" title="Edit Ulasan" data-id="${r.id}">✏️</button>
            <button type="button" class="btn-table-action btn-toggle-review" title="${isVisible ? 'Sembunyikan' : 'Tampilkan'}" data-id="${r.id}">${isVisible ? '👁️' : '🙈'}</button>
            <button type="button" class="btn-table-action btn-del-review" style="color: #f87171;" title="Hapus Ulasan" data-id="${r.id}">🗑️</button>
          </div>
        </td>
      </tr>
    `;
  }).join('');
}

function openCreateReviewModal() {
  document.getElementById('createReviewForm')?.reset();
  const vis = document.getElementById('crIsVisible');
  if (vis) vis.checked = true;
  document.getElementById('createReviewModal')?.classList.add('open');
}

function closeCreateReviewModal() {
  document.getElementById('createReviewModal')?.classList.remove('open');
}
window.openCreateReviewModal = openCreateReviewModal;
window.closeCreateReviewModal = closeCreateReviewModal;

async function submitCreateReview(e) {
  e.preventDefault();
  const slug = document.getElementById('crProductSlug')?.value;
  if (!slug) {
    alert('Pilih produk terlebih dahulu');
    return;
  }

  const author = (document.getElementById('crAuthorName')?.value || '').trim() || 'Pelanggan Terverifikasi';
  const rating = parseInt(document.getElementById('crRating')?.value, 10) || 5;
  const text = (document.getElementById('crReviewText')?.value || '').trim();
  const isVisible = !!document.getElementById('crIsVisible')?.checked;

  const btn = document.getElementById('submitCreateReviewBtn');
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  try {
    const res = await adminFetch('/api/admin/reviews', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        product_slug: slug,
        author_name: author,
        rating: rating,
        review_text: text,
        is_visible: isVisible
      })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Ulasan berhasil ditambahkan! ⭐');
      closeCreateReviewModal();
      await loadAdminReviews();
    } else {
      alert('Gagal menambahkan ulasan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Simpan Ulasan';
  }
}

function openEditReviewModal(id) {
  const rev = allAdminReviews.find(r => r.id === id);
  if (!rev) return;

  document.getElementById('erReviewId').value = rev.id;
  document.getElementById('erProductTitle').value = rev.product_slug;
  document.getElementById('erAuthorName').value = rev.author_name || '';
  document.getElementById('erRating').value = String(rev.rating || 5);
  document.getElementById('erReviewText').value = rev.review_text || '';
  document.getElementById('erIsVisible').checked = !!rev.is_visible;

  document.getElementById('editReviewModal')?.classList.add('open');
}

function closeEditReviewModal() {
  document.getElementById('editReviewModal')?.classList.remove('open');
}
window.openEditReviewModal = openEditReviewModal;
window.closeEditReviewModal = closeEditReviewModal;

async function submitEditReview(e) {
  e.preventDefault();
  const id = parseInt(document.getElementById('erReviewId')?.value, 10);
  if (!id) return;

  const author = (document.getElementById('erAuthorName')?.value || '').trim();
  const rating = parseInt(document.getElementById('erRating')?.value, 10) || 5;
  const text = (document.getElementById('erReviewText')?.value || '').trim();
  const isVisible = !!document.getElementById('erIsVisible')?.checked;

  const btn = document.getElementById('submitEditReviewBtn');
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  try {
    const res = await adminFetch(`/api/admin/reviews/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        author_name: author,
        rating: rating,
        review_text: text,
        is_visible: isVisible
      })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Ulasan berhasil diperbarui! ✏️');
      closeEditReviewModal();
      await loadAdminReviews();
    } else {
      alert('Gagal memperbarui ulasan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Simpan Perubahan';
  }
}

async function toggleReviewVisibility(id) {
  const rev = allAdminReviews.find(r => r.id === id);
  if (!rev) return;

  const newVis = !rev.is_visible;
  try {
    const res = await adminFetch(`/api/admin/reviews/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        author_name: rev.author_name,
        rating: rev.rating,
        review_text: rev.review_text,
        is_visible: newVis
      })
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      rev.is_visible = newVis;
      showToast(newVis ? 'Ulasan ditampilkan di etalase 👁️' : 'Ulasan disembunyikan 🙈');
      renderReviewsTable();
      const visCount = allAdminReviews.filter(r => r.is_visible).length;
      const cntVisEl = document.getElementById('cntVisibleReviews');
      if (cntVisEl) cntVisEl.textContent = visCount;
    } else {
      alert('Gagal mengubah status visibilitas: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  }
}

async function deleteReview(id) {
  if (!confirm(`Hapus ulasan #${id} secara permanen? Tindakan ini tidak dapat dibatalkan.`)) return;

  try {
    const res = await adminFetch(`/api/admin/reviews/${id}`, {
      method: 'DELETE'
    });

    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Ulasan berhasil dihapus 🗑️');
      await loadAdminReviews();
    } else {
      alert('Gagal menghapus ulasan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Terjadi kesalahan: ' + err.message);
  }
}

function setupReviewsPageUI() {
  // Sub-tabs switching
  const tabBtnEtalase = document.getElementById('tabBtnEtalase');
  const tabBtnReviews = document.getElementById('tabBtnReviews');
  const secEtalase = document.getElementById('sectionEtalase');
  const secReviews = document.getElementById('sectionReviews');

  tabBtnEtalase?.addEventListener('click', () => {
    tabBtnEtalase.classList.add('active');
    tabBtnReviews?.classList.remove('active');
    if (secEtalase) secEtalase.style.display = 'block';
    if (secReviews) secReviews.style.display = 'none';
  });

  tabBtnReviews?.addEventListener('click', () => {
    tabBtnReviews.classList.add('active');
    tabBtnEtalase?.classList.remove('active');
    if (secReviews) secReviews.style.display = 'block';
    if (secEtalase) secEtalase.style.display = 'none';
  });

  // Refresh button
  document.getElementById('btnRefreshReviewsPage')?.addEventListener('click', () => {
    loadEtalaseProducts();
    loadAdminReviews();
    showToast('Data ulasan & etalase diperbarui 🔄');
  });

  // Event Delegation for Etalase Save Button
  document.getElementById('etalaseTableBody')?.addEventListener('click', (e) => {
    const btn = e.target.closest('.btn-save-etalase');
    if (!btn) return;
    const slug = btn.dataset.slug;
    const row = btn.closest('tr');
    saveProductEtalaseStats(slug, row, btn);
  });

  // Event Delegation for Reviews Action Buttons (Edit, Toggle, Delete)
  document.getElementById('reviewsTableBody')?.addEventListener('click', (e) => {
    const btnEdit = e.target.closest('.btn-edit-review');
    if (btnEdit) {
      openEditReviewModal(parseInt(btnEdit.dataset.id, 10));
      return;
    }
    const btnToggle = e.target.closest('.btn-toggle-review');
    if (btnToggle) {
      toggleReviewVisibility(parseInt(btnToggle.dataset.id, 10));
      return;
    }
    const btnDel = e.target.closest('.btn-del-review');
    if (btnDel) {
      deleteReview(parseInt(btnDel.dataset.id, 10));
      return;
    }
  });

  // Search etalase
  document.getElementById('searchEtalaseInput')?.addEventListener('input', applyEtalaseFilter);

  // Search & Filter reviews
  document.getElementById('searchReviewInput')?.addEventListener('input', applyReviewFilter);
  document.getElementById('filterReviewProductSelect')?.addEventListener('change', applyReviewFilter);

  // Create review modal
  document.getElementById('btnOpenCreateReviewModal')?.addEventListener('click', openCreateReviewModal);
  document.getElementById('closeCreateReviewBtn')?.addEventListener('click', closeCreateReviewModal);
  document.getElementById('cancelCreateReviewBtn')?.addEventListener('click', closeCreateReviewModal);
  document.getElementById('createReviewForm')?.addEventListener('submit', submitCreateReview);

  // Edit review modal
  document.getElementById('closeEditReviewBtn')?.addEventListener('click', closeEditReviewModal);
  document.getElementById('cancelEditReviewBtn')?.addEventListener('click', closeEditReviewModal);
  document.getElementById('editReviewForm')?.addEventListener('submit', submitEditReview);

  // Backdrop clicks
  document.getElementById('createReviewModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'createReviewModal') closeCreateReviewModal();
  });
  document.getElementById('editReviewModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'editReviewModal') closeEditReviewModal();
  });
}

// -------------------------------------------------------------
// 11. MULTILINGUAL TRANSLATIONS MANAGEMENT LOGIC
// -------------------------------------------------------------
let allTranslations = [];
let currentTransFilter = 'ALL';

async function loadTranslations() {
  const tbody = document.getElementById('translationsTableBody');
  if (tbody) {
    tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 2rem; color: var(--text-muted);">Memuat data terjemahan produk...</td></tr>`;
  }

  try {
    const res = await adminFetch('/api/admin/translations');
    if (!res) return;

    const json = await res.json();
    if (json.success && Array.isArray(json.data)) {
      allTranslations = json.data;
      updateTranslationCounters();
      renderTranslationsTable();
    } else {
      throw new Error(json.error || 'Respon API tidak valid');
    }
  } catch (err) {
    console.error('Failed to load translations:', err);
    if (tbody) {
      tbody.innerHTML = `<tr><td colspan="6" style="text-align: center; padding: 2rem; color: #f87171;">✕ ${escapeHtml(err.message || 'Gagal memuat data')}</td></tr>`;
    }
  }
}

function updateTranslationCounters() {
  const total = allTranslations.length;
  const translated = allTranslations.filter(t => t.is_translated).length;
  const pending = total - translated;

  const elTotal = document.getElementById('cntTotalProds');
  if (elTotal) elTotal.textContent = total;
  const elTrans = document.getElementById('cntTranslatedProds');
  if (elTrans) elTrans.textContent = translated;
  const elPending = document.getElementById('cntPendingProds');
  if (elPending) elPending.textContent = pending;
}

function renderTranslationsTable() {
  const tbody = document.getElementById('translationsTableBody');
  if (!tbody) return;

  const q = (document.getElementById('searchTransInput')?.value || '').toLowerCase().trim();

  const filtered = allTranslations.filter(t => {
    if (currentTransFilter === 'TRANSLATED' && !t.is_translated) return false;
    if (currentTransFilter === 'PENDING' && t.is_translated) return false;

    if (q) {
      const matchName = (t.name_id || '').toLowerCase().includes(q) || (t.name_en || '').toLowerCase().includes(q);
      const matchSlug = (t.slug || '').toLowerCase().includes(q);
      const matchType = (t.type_id || '').toLowerCase().includes(q);
      return matchName || matchSlug || matchType;
    }
    return true;
  });

  if (filtered.length === 0) {
    tbody.innerHTML = `
      <tr>
        <td colspan="6" style="text-align: center; padding: 2.5rem; color: var(--text-muted);">
          Tidak ada data produk yang cocok dengan pencarian / filter ini.
        </td>
      </tr>
    `;
    return;
  }

  tbody.innerHTML = filtered.map((t, idx) => {
    const badge = t.is_translated
      ? `<span class="trans-status-badge badge-trans-yes">✓ Sudah di-set (EN)</span>`
      : `<span class="trans-status-badge badge-trans-no">⚠️ Belum di-set</span>`;

    const enPreview = (t.name_en && t.name_en.trim())
      ? `<span style="color: #cbd5e1; font-weight: 500;">${escapeHtml(t.name_en)}</span>`
      : `<span style="color: #94a3b8; font-size: 0.8rem; font-style: italic;">Belum di-set (Pakai Teks Indo)</span>`;

    let sourceBadge = `<span style="font-size: 0.7rem; padding: 1px 5px; border-radius: 4px; background: rgba(168,85,247,0.15); color: #c084fc; margin-left: 6px;">DC</span>`;
    if (t.source === 'LOCAL') {
      sourceBadge = `<span style="font-size: 0.7rem; padding: 1px 5px; border-radius: 4px; background: rgba(59,130,246,0.15); color: #60a5fa; margin-left: 6px;">Lokal</span>`;
    } else if (t.source === 'SILVEX') {
      sourceBadge = `<span style="font-size: 0.7rem; padding: 1px 5px; border-radius: 4px; background: rgba(16,185,129,0.15); color: #34d399; margin-left: 6px;">Silvex</span>`;
    }

    return `
      <tr>
        <td style="color: var(--text-muted); font-size: 0.85rem;">${idx + 1}</td>
        <td>
          <div style="font-weight: 600; color: #fff; display: flex; align-items: center;">
            <span>${escapeHtml(t.name_id)}</span>
            ${sourceBadge}
          </div>
          <div style="font-size: 0.78rem; color: var(--text-muted); font-family: monospace;">slug: ${escapeHtml(t.slug)}</div>
        </td>
        <td style="font-size: 0.88rem; color: var(--text-muted);">${escapeHtml(t.type_id || 'Digital License')}</td>
        <td>${badge}</td>
        <td>${enPreview}</td>
        <td style="text-align: right;">
          <button type="button" class="btn-nav" style="padding: 0.35rem 0.75rem; font-size: 0.82rem; border-color: rgba(99,102,241,0.4); color: #a5b4fc;" onclick="openTransModal('${escapeHtml(t.slug)}')">
            ✏️ Edit EN
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

function openTransModal(slug) {
  const item = allTranslations.find(t => t.slug === slug);
  if (!item) return;

  const slugInput = document.getElementById('transProductSlug');
  if (slugInput) slugInput.value = item.slug;

  const badgeEl = document.getElementById('modalProductSlugBadge');
  if (badgeEl) badgeEl.textContent = 'slug: ' + item.slug + ' (' + item.source + ')';

  // Original ID
  const elOrigName = document.getElementById('origNameID');
  if (elOrigName) elOrigName.value = item.name_id || '';
  const elOrigType = document.getElementById('origTypeID');
  if (elOrigType) elOrigType.value = item.type_id || '';
  const elOrigReg = document.getElementById('origRegionID');
  if (elOrigReg) elOrigReg.value = item.region_id || '';
  const elOrigWar = document.getElementById('origWarrantyID');
  if (elOrigWar) elOrigWar.value = item.warranty_id || '';
  const elOrigDesc = document.getElementById('origDescID');
  if (elOrigDesc) elOrigDesc.value = item.description_id || '';
  const elOrigTerms = document.getElementById('origTermsID');
  if (elOrigTerms) elOrigTerms.value = item.terms_id || '';

  // English Fields
  const elTransName = document.getElementById('transNameEN');
  if (elTransName) elTransName.value = item.name_en || '';
  const elTransType = document.getElementById('transTypeEN');
  if (elTransType) elTransType.value = item.type_en || '';
  const elTransReg = document.getElementById('transRegionEN');
  if (elTransReg) elTransReg.value = item.region_en || '';
  const elTransWar = document.getElementById('transWarrantyEN');
  if (elTransWar) elTransWar.value = item.warranty_en || '';
  const elTransDesc = document.getElementById('transDescEN');
  if (elTransDesc) elTransDesc.value = item.description_en || '';
  const elTransTerms = document.getElementById('transTermsEN');
  if (elTransTerms) elTransTerms.value = item.terms_en || '';

  const modal = document.getElementById('editTransModal');
  modal?.classList.add('open');
  document.body.style.overflow = 'hidden';

  setTimeout(() => {
    document.getElementById('transNameEN')?.focus();
  }, 100);
}

function closeTransModal() {
  const modal = document.getElementById('editTransModal');
  modal?.classList.remove('open');
  document.body.style.overflow = '';
}

async function handleAutoDraft() {
  const btn = document.getElementById('btnAutoDraft');
  if (!btn) return;
  const origText = btn.innerHTML;
  btn.disabled = true;
  btn.innerHTML = `<span>⏳ Menerjemahkan...</span>`;

  try {
    const payload = {
      texts: {
        name: document.getElementById('origNameID')?.value || '',
        type: document.getElementById('origTypeID')?.value || '',
        region: document.getElementById('origRegionID')?.value || '',
        warranty: document.getElementById('origWarrantyID')?.value || '',
        description: document.getElementById('origDescID')?.value || '',
        terms: document.getElementById('origTermsID')?.value || '',
      }
    };

    const res = await adminFetch('/api/admin/translations/auto-draft', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res) return;

    const json = await res.json();
    if (json.success && json.data) {
      if (json.data.name && document.getElementById('transNameEN')) document.getElementById('transNameEN').value = json.data.name;
      if (json.data.type && document.getElementById('transTypeEN')) document.getElementById('transTypeEN').value = json.data.type;
      if (json.data.region && document.getElementById('transRegionEN')) document.getElementById('transRegionEN').value = json.data.region;
      if (json.data.warranty && document.getElementById('transWarrantyEN')) document.getElementById('transWarrantyEN').value = json.data.warranty;
      if (json.data.description && document.getElementById('transDescEN')) document.getElementById('transDescEN').value = json.data.description;
      if (json.data.terms && document.getElementById('transTermsEN')) document.getElementById('transTermsEN').value = json.data.terms;

      showToast('✨ Draft terjemahan Bahasa Inggris berhasil digenerate!');
    } else {
      alert('Gagal generate draft terjemahan: ' + (json.error || 'Terjadi kesalahan'));
    }
  } catch (err) {
    alert('Terjadi kesalahan jaringan saat auto-draft: ' + err.message);
  } finally {
    btn.disabled = false;
    btn.innerHTML = origText;
  }
}

async function handleSaveTranslation(e) {
  if (e && e.preventDefault) e.preventDefault();
  const slug = document.getElementById('transProductSlug')?.value;
  if (!slug) return;

  const btn = document.getElementById('btnSaveTranslation');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Menyimpan...';
  }

  const payload = {
    name_en: (document.getElementById('transNameEN')?.value || '').trim(),
    type_en: (document.getElementById('transTypeEN')?.value || '').trim(),
    region_en: (document.getElementById('transRegionEN')?.value || '').trim(),
    warranty_en: (document.getElementById('transWarrantyEN')?.value || '').trim(),
    description_en: (document.getElementById('transDescEN')?.value || '').trim(),
    terms_en: (document.getElementById('transTermsEN')?.value || '').trim()
  };

  try {
    const res = await adminFetch('/api/admin/translations/' + encodeURIComponent(slug), {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res) return;

    const json = await res.json();
    if (json.success) {
      closeTransModal();
      await loadTranslations();
      showToast('✅ Terjemahan bahasa Inggris untuk "' + slug + '" berhasil disimpan!');
    } else {
      alert('✕ Gagal menyimpan terjemahan: ' + (json.error || 'Terjadi kesalahan'));
    }
  } catch (err) {
    alert('✕ Kesalahan jaringan: ' + err.message);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = '💾 Simpan Terjemahan';
    }
  }
}

function setupTranslationsUI() {
  document.getElementById('btnRefreshTranslations')?.addEventListener('click', () => {
    loadTranslations();
    showToast('Data terjemahan diperbarui 🔄');
  });

  document.getElementById('searchTransInput')?.addEventListener('input', renderTranslationsTable);

  document.querySelectorAll('#transFilterGroup button').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('#transFilterGroup button').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentTransFilter = btn.getAttribute('data-tfilter') || 'ALL';
      renderTranslationsTable();
    });
  });

  // Modal events
  document.getElementById('btnCloseEditTransModal')?.addEventListener('click', closeTransModal);
  document.getElementById('btnCancelTransModal')?.addEventListener('click', closeTransModal);
  document.getElementById('editTransModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'editTransModal') closeTransModal();
  });

  // Form submit
  document.getElementById('transForm')?.addEventListener('submit', handleSaveTranslation);

  // Auto Draft button
  document.getElementById('btnAutoDraft')?.addEventListener('click', handleAutoDraft);
}

// Expose modal handlers to global scope for HTML onclick attributes
window.openTransModal = openTransModal;
window.closeTransModal = closeTransModal;

// -------------------------------------------------------------
// 14. SILVEX AI PRODUCT MANAGEMENT & LOGO PRESETS
// -------------------------------------------------------------
let allSilvexProducts = [];
let filteredSilvexProducts = [];
let currentSilvexFilter = 'ALL';
let currentEditingSilvexSlug = null;

const PRESET_AI_LOGOS = [
  { name: 'ChatGPT', url: 'https://api.iconify.design/simple-icons:openai.svg?color=%2310a37f' },
  { name: 'Claude', url: 'https://api.iconify.design/simple-icons:anthropic.svg?color=%23d97706' },
  { name: 'Midjourney', url: 'https://api.iconify.design/simple-icons:midjourney.svg?color=%23ffffff' },
  { name: 'Canva', url: 'https://api.iconify.design/simple-icons:canva.svg?color=%2300c4cc' },
  { name: 'Cursor', url: 'https://api.iconify.design/simple-icons:cursor.svg?color=%2338bdf8' },
  { name: 'Perplexity', url: 'https://api.iconify.design/simple-icons:perplexity.svg?color=%2320b2aa' },
  { name: 'Gemini', url: 'https://api.iconify.design/simple-icons:googlegemini.svg?color=%234285f4' },
  { name: 'Copilot', url: 'https://api.iconify.design/simple-icons:githubcopilot.svg?color=%23ffffff' },
  { name: 'ElevenLabs', url: 'https://api.iconify.design/simple-icons:elevenlabs.svg?color=%23ffffff' },
  { name: 'Suno AI', url: 'https://api.iconify.design/simple-icons:suno.svg?color=%23f43f5e' },
  { name: 'Runway', url: 'https://api.iconify.design/simple-icons:runway.svg?color=%236366f1' },
  { name: 'DeepL', url: 'https://api.iconify.design/simple-icons:deepl.svg?color=%230f2b46' },
  { name: 'Grammarly', url: 'https://api.iconify.design/simple-icons:grammarly.svg?color=%2315c39a' },
  { name: 'CapCut', url: 'https://api.iconify.design/simple-icons:capcut.svg?color=%23ffffff' },
  { name: 'Netflix', url: 'https://api.iconify.design/simple-icons:netflix.svg?color=%23e50914' },
  { name: 'Spotify', url: 'https://api.iconify.design/simple-icons:spotify.svg?color=%231ed760' },
  { name: 'YouTube', url: 'https://api.iconify.design/simple-icons:youtube.svg?color=%23ff0000' }
];

async function loadSilvexProducts() {
  const tbody = document.getElementById('adminSilvexTableBody');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: var(--text-muted);">Memuat katalog produk Silvex AI...</td></tr>`;

  try {
    const res = await adminFetch('/api/admin/silvex/products');
    if (!res) return;
    const json = await res.json();
    if (!json.success || !json.data) {
      tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: #f87171;">Gagal memuat produk Silvex: ${escapeHtml(json.error || 'Server error')}</td></tr>`;
      return;
    }

    allSilvexProducts = json.data;
    updateSilvexCounters();
    applySilvexFilter();

    // Fetch balance for Silvex card
    adminFetch('/api/admin/silvex/check').then(async (cRes) => {
      if (!cRes) return;
      const cJson = await cRes.json();
      if (cJson.success && cJson.data && cJson.data.success) {
        const bal = cJson.data.balance || 0;
        const balEl = document.getElementById('cntSilvexBalance');
        if (balEl) balEl.textContent = `$${bal.toFixed(2)}`;
      }
    }).catch(() => {});

  } catch (err) {
    console.error('Failed to load Silvex products:', err);
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: #f87171;">Error: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function updateSilvexCounters() {
  const total = allSilvexProducts.length;
  const active = allSilvexProducts.filter(p => p.is_active).length;
  const hidden = total - active;

  if (document.getElementById('cntTotalSilvex')) document.getElementById('cntTotalSilvex').textContent = total;
  if (document.getElementById('cntActiveSilvex')) document.getElementById('cntActiveSilvex').textContent = active;
  if (document.getElementById('cntHiddenSilvex')) document.getElementById('cntHiddenSilvex').textContent = hidden;
  if (document.getElementById('totalSilvexBadge')) document.getElementById('totalSilvexBadge').textContent = `● ${active} Aktif dari ${total} Produk Silvex`;
}

function applySilvexFilter() {
  const search = (document.getElementById('searchSilvexInput')?.value || '').trim().toLowerCase();

  filteredSilvexProducts = allSilvexProducts.filter(p => {
    const match = (p.name || '').toLowerCase().includes(search) ||
                  (p.original_name || '').toLowerCase().includes(search) ||
                  (p.slug || '').toLowerCase().includes(search) ||
                  (p.type || '').toLowerCase().includes(search);
    if (!match) return false;

    if (currentSilvexFilter === 'ALL') return true;
    if (currentSilvexFilter === 'ACTIVE') return p.is_active;
    if (currentSilvexFilter === 'HIDDEN') return !p.is_active;
    if (currentSilvexFilter === 'CUSTOM') return p.is_customized;
    return true;
  });

  renderSilvexProductsTable();
}

function renderSilvexProductsTable() {
  const tbody = document.getElementById('adminSilvexTableBody');
  if (!tbody) return;

  if (filteredSilvexProducts.length === 0) {
    tbody.innerHTML = `<tr><td colspan="7" style="text-align:center; padding: 2.5rem; color: var(--text-muted);">Tidak ada produk Silvex yang cocok dengan filter.</td></tr>`;
    return;
  }

  tbody.innerHTML = filteredSilvexProducts.map(p => {
    const isCustomPrice = p.custom_price_idr > 0;
    const isCustomTitle = !!p.custom_name;
    const logoSrc = p.custom_logo || p.logo || '/logo/digimest_petak.png';

    return `
      <tr>
        <td>
          <div style="display: flex; align-items: center; gap: 10px;">
            <img src="${logoSrc}" alt="" style="width: 38px; height: 38px; border-radius: 8px; object-fit: contain; background: #0f172a; padding: 3px; border: 1px solid var(--border-subtle);" onerror="this.src='/logo/digimest_petak.png'">
            <div>
              <div style="font-weight: 700; color: #fff;">${escapeHtml(p.name)}</div>
              ${isCustomTitle ? `<div style="font-size: 0.72rem; color: var(--text-muted);">Asli: ${escapeHtml(p.original_name)}</div>` : ''}
              <div style="font-size: 0.72rem; color: #38bdf8; font-family: var(--font-mono);">${escapeHtml(p.slug)}</div>
            </div>
          </div>
        </td>
        <td>
          <div style="font-weight: 600;">${p.stock} pcs ${p.custom_stock !== undefined && p.custom_stock >= 0 ? '<span class="badge-custom" style="font-size: 0.65rem; padding: 1px 6px; margin-left: 4px;">Kustom</span>' : ''}</div>
          <div style="font-size: 0.75rem; color: var(--text-muted);">${escapeHtml(p.region || 'Global')} • ${escapeHtml(p.warranty || '-')}</div>
        </td>
        <td>
          <div style="font-family: var(--font-mono); color: #fbbf24; font-weight: 700;">$${(p.original_price_usd || 0).toFixed(2)}</div>
          <div style="font-size: 0.72rem; color: #94a3b8;">(Modal: ${formatIDR(p.modal_idr)})</div>
          ${p.surge_percent > 0 ? `<div style="font-size: 0.7rem; color: #f59e0b; font-weight: 600;">+${p.surge_percent.toFixed(1)}% naik</div>` : ''}
        </td>
        <td>
          <div style="font-weight: 700; color: #fff; font-family: var(--font-mono);">${formatIDR(p.reference_price_idr)}</div>
          <div style="font-size: 0.72rem; color: var(--text-muted);">
            ${isCustomPrice ? '<span class="badge-custom">Kustom</span>' : '<span class="badge-auto">Formula Env</span>'}
          </div>
        </td>
        <td>
          ${p.has_promo ? `
            <div style="font-weight: 700; color: #fbbf24; font-family: var(--font-mono);">${formatIDR(p.promo_price_idr)}</div>
            <div style="font-size: 0.72rem;"><span class="badge-discount-mini">-${p.discount_percent}% OFF</span></div>
          ` : `
            <span style="color: var(--text-muted); font-size: 0.85rem;">-</span>
          `}
        </td>
        <td>
          ${p.is_auto_suspended 
            ? `<span class="status-pill failed" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" title="Modal upstream melonjak melebihi ambang batas.">⚠️ Disuspend (+${p.surge_percent.toFixed(0)}%)</span>` 
            : (!p.is_active 
                ? '<span class="status-pill failed">● Tersembunyi</span>' 
                : (p.is_force_out_of_stock
                    ? '<span class="status-pill warning" style="background: rgba(245,158,11,0.18); color: #fbbf24; border: 1px solid rgba(245,158,11,0.35);" title="Produk dipaksa habis secara manual oleh admin.">⚡ Paksa Habis</span>'
                    : (p.stock <= 0
                        ? '<span class="status-pill" style="background: rgba(148,163,184,0.12); color: #94a3b8; border: 1px solid rgba(148,163,184,0.25);" title="Stok asli dari Silvex AI kosong.">● Habis (Silvex)</span>'
                        : '<span class="status-pill paid">● Aktif</span>')))}
        </td>
        <td style="text-align: right;">
          <button class="btn-action-mini" onclick="openEditSilvexModal('${escapeHtml(p.slug)}')">
            ✏️ Edit
          </button>
        </td>
      </tr>
    `;
  }).join('');
}

function renderSilvexLogoPresets() {
  const container = document.getElementById('silvexLogoPresetButtons');
  if (!container) return;

  container.innerHTML = PRESET_AI_LOGOS.map(p => `
    <button type="button" class="btn-nav silvex-preset-btn" data-url="${escapeHtml(p.url)}" style="display: inline-flex; align-items: center; gap: 6px; font-size: 0.74rem; padding: 4px 8px; border-radius: 6px; cursor: pointer; background: rgba(255,255,255,0.04); border: 1px solid var(--border-subtle);">
      <img src="${escapeHtml(p.url)}" alt="" style="width: 14px; height: 14px; object-fit: contain;">
      <span>${escapeHtml(p.name)}</span>
    </button>
  `).join('');

  container.querySelectorAll('.silvex-preset-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const url = btn.getAttribute('data-url');
      const input = document.getElementById('edSilvexCustomLogo');
      const preview = document.getElementById('edSilvexLogoPreview');
      if (input) input.value = url;
      if (preview) preview.src = url;
      showToast(`Preset icon "${btn.textContent.trim()}" dipilih ✨`);
    });
  });
}

function openEditSilvexModal(slug) {
  const p = allSilvexProducts.find(item => item.slug === slug);
  if (!p) return;

  currentEditingSilvexSlug = slug;

  const currentLogo = p.custom_logo || p.logo || '/logo/digimest_petak.png';
  document.getElementById('edSilvexLogoPreview').src = currentLogo;
  document.getElementById('edSilvexOriginalName').textContent = p.original_name;
  document.getElementById('edSilvexSlug').textContent = p.slug;
  document.getElementById('edSilvexUSD').textContent = `$${(p.original_price_usd || 0).toFixed(2)}`;
  document.getElementById('edSilvexModalIDR').textContent = `Modal: ${formatIDR(p.modal_idr)}`;
  document.getElementById('edSilvexEnvPrice').textContent = formatIDR(p.env_price_idr);

  document.getElementById('edSilvexIsActive').checked = p.is_active;
  const forceOutEl = document.getElementById('edSilvexIsForceOutOfStock');
  if (forceOutEl) forceOutEl.checked = !!p.is_force_out_of_stock;

  // Custom Logo
  const edLogo = document.getElementById('edSilvexCustomLogo');
  if (edLogo) edLogo.value = p.custom_logo || '';

  document.getElementById('edSilvexCustomName').value = p.custom_name || '';

  // Custom Type
  const edType = document.getElementById('edSilvexCustomType');
  if (edType) edType.value = p.custom_type || '';
  const defTypeEl = document.getElementById('edSilvexDefaultTypeNotice');
  if (defTypeEl) defTypeEl.textContent = p.original_type || p.type || 'AI Account';

  // Custom Region
  const edReg = document.getElementById('edSilvexCustomRegion');
  if (edReg) edReg.value = p.custom_region || '';
  const defRegEl = document.getElementById('edSilvexDefaultRegionNotice');
  if (defRegEl) defRegEl.textContent = p.original_region || p.region || 'Global';

  // Custom Warranty
  const edWar = document.getElementById('edSilvexCustomWarranty');
  if (edWar) edWar.value = p.custom_warranty || '';
  const defWarEl = document.getElementById('edSilvexDefaultWarrantyNotice');
  if (defWarEl) defWarEl.textContent = p.original_warranty || p.warranty || 'Garansi Toko';

  // Custom Stock
  const edStock = document.getElementById('edSilvexCustomStock');
  if (edStock) edStock.value = (p.custom_stock !== undefined && p.custom_stock >= 0) ? p.custom_stock : '';
  const defStockEl = document.getElementById('edSilvexDefaultStockNotice');
  if (defStockEl) defStockEl.textContent = `${p.original_stock !== undefined ? p.original_stock : p.stock} Stok`;

  // Pre-fill Reference Price
  const refPrice = p.reference_price_idr || p.calculated_price_idr;
  document.getElementById('edSilvexCustomPrice').value = refPrice;
  document.getElementById('edSilvexMinPriceNotice').textContent = formatIDR(p.env_price_idr);

  // Pre-fill Promo Price
  document.getElementById('edSilvexPromoPrice').value = p.promo_price_idr > 0 ? p.promo_price_idr : '';
  document.getElementById('edSilvexMinPromoNotice').textContent = formatIDR(p.modal_idr);

  document.getElementById('edSilvexCustomDesc').value = p.custom_description || '';
  document.getElementById('edSilvexCustomTerms').value = p.custom_terms || (p.terms || 'Membeli berarti setuju');

  document.getElementById('editSilvexModal')?.classList.add('open');
}

function closeEditSilvexModal() {
  document.getElementById('editSilvexModal')?.classList.remove('open');
  currentEditingSilvexSlug = null;
}

async function saveSilvexProductOverride() {
  if (!currentEditingSilvexSlug) return;
  const p = allSilvexProducts.find(item => item.slug === currentEditingSilvexSlug);
  if (!p) return;

  const customPrice = parseInt(document.getElementById('edSilvexCustomPrice').value, 10) || 0;
  const promoPrice = parseInt(document.getElementById('edSilvexPromoPrice').value, 10) || 0;

  if (customPrice > 0 && customPrice < p.env_price_idr) {
    alert(`Harga acuan (${formatIDR(customPrice)}) tidak boleh lebih rendah dari harga default sistem (${formatIDR(p.env_price_idr)})`);
    return;
  }

  if (promoPrice > 0) {
    if (promoPrice < p.modal_idr) {
      alert(`Harga promo (${formatIDR(promoPrice)}) tidak boleh lebih rendah dari modal toko (${formatIDR(p.modal_idr)})`);
      return;
    }
    const effectiveRef = customPrice > 0 ? customPrice : p.env_price_idr;
    if (promoPrice >= effectiveRef) {
      alert(`Harga promo (${formatIDR(promoPrice)}) harus lebih murah dari harga acuan (${formatIDR(effectiveRef)})`);
      return;
    }
  }

  const customStockVal = document.getElementById('edSilvexCustomStock') ? document.getElementById('edSilvexCustomStock').value.trim() : '';
  let customStock = -1;
  if (customStockVal !== '') {
    const parsedStock = parseInt(customStockVal, 10);
    if (!isNaN(parsedStock) && parsedStock >= 0) {
      customStock = parsedStock;
    }
  }

  const btn = document.getElementById('btnSaveSilvexProduct');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Menyimpan...';
  }

  const body = {
    is_active: document.getElementById('edSilvexIsActive').checked,
    is_force_out_of_stock: document.getElementById('edSilvexIsForceOutOfStock') ? document.getElementById('edSilvexIsForceOutOfStock').checked : false,
    custom_logo: (document.getElementById('edSilvexCustomLogo')?.value || '').trim(),
    custom_name: (document.getElementById('edSilvexCustomName').value || '').trim(),
    custom_type: (document.getElementById('edSilvexCustomType')?.value || '').trim(),
    custom_region: (document.getElementById('edSilvexCustomRegion')?.value || '').trim(),
    custom_warranty: (document.getElementById('edSilvexCustomWarranty')?.value || '').trim(),
    custom_stock: customStock,
    custom_price_idr: customPrice,
    promo_price_idr: promoPrice,
    custom_description: (document.getElementById('edSilvexCustomDesc').value || '').trim(),
    custom_terms: (document.getElementById('edSilvexCustomTerms').value || '').trim()
  };

  try {
    const res = await adminFetch(`/api/admin/products/${encodeURIComponent(currentEditingSilvexSlug)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Kustomisasi produk Silvex AI berhasil disimpan! ✨');
      closeEditSilvexModal();
      await loadSilvexProducts();
    } else {
      alert('Gagal menyimpan: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.textContent = 'Simpan Kustomisasi';
    }
  }
}

async function resetSilvexProductOverride() {
  if (!currentEditingSilvexSlug) return;
  if (!confirm(`Reset kustomisasi produk "${currentEditingSilvexSlug}" ke pengaturan bawaan Silvex AI?`)) return;

  const btn = document.getElementById('btnResetSilvexProduct');
  if (btn) btn.disabled = true;

  try {
    const res = await adminFetch(`/api/admin/products/${encodeURIComponent(currentEditingSilvexSlug)}/reset`, {
      method: 'POST'
    });
    if (!res) return;
    const json = await res.json();
    if (json.success) {
      showToast('Produk Silvex AI dikembalikan ke default 🔄');
      closeEditSilvexModal();
      await loadSilvexProducts();
    } else {
      alert('Gagal mereset: ' + (json.error || 'Server error'));
    }
  } catch (err) {
    alert('Error: ' + err.message);
  } finally {
    if (btn) btn.disabled = false;
  }
}

function setupSilvexUI() {
  renderSilvexLogoPresets();

  document.querySelectorAll('[data-sfilter]').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('[data-sfilter]').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      currentSilvexFilter = e.target.dataset.sfilter;
      applySilvexFilter();
    });
  });

  document.getElementById('searchSilvexInput')?.addEventListener('input', applySilvexFilter);

  // Logo live preview & clear button
  const logoInput = document.getElementById('edSilvexCustomLogo');
  const previewImg = document.getElementById('edSilvexLogoPreview');
  logoInput?.addEventListener('input', (e) => {
    const val = e.target.value.trim();
    if (previewImg) previewImg.src = val || '/logo/digimest_petak.png';
  });

  document.getElementById('btnClearSilvexLogo')?.addEventListener('click', () => {
    if (logoInput) logoInput.value = '';
    if (previewImg) {
      const p = allSilvexProducts.find(item => item.slug === currentEditingSilvexSlug);
      previewImg.src = p?.logo || '/logo/digimest_petak.png';
    }
    showToast('Logo kustom dibersihkan, menggunakan deteksi otomatis');
  });

  // Modal events
  document.getElementById('closeSilvexModalBtn')?.addEventListener('click', closeEditSilvexModal);
  document.getElementById('btnCancelSilvexEdit')?.addEventListener('click', closeEditSilvexModal);
  document.getElementById('btnSaveSilvexProduct')?.addEventListener('click', saveSilvexProductOverride);
  document.getElementById('btnResetSilvexProduct')?.addEventListener('click', resetSilvexProductOverride);
}

// Expose Silvex modal handlers to window for onclick attributes
window.openEditSilvexModal = openEditSilvexModal;
window.closeEditSilvexModal = closeEditSilvexModal;

