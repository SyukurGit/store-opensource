// StoreKu Admin Engine (Multi-page Support)

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
  const token = localStorage.getItem('storeku_admin_token');
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
  const res = await fetch(url, { ...options, headers: combinedHeaders });
  if (res.status === 401) {
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
    document.getElementById('statDcBalance').textContent = `$${(s.digitalcore_balance || 0).toFixed(2)}`;
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
          <div style="font-weight: 600;">${p.stock} pcs</div>
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
            : (p.is_active 
                ? '<span class="status-pill paid">● Aktif</span>' 
                : '<span class="status-pill failed">● Tersembunyi</span>')}
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
  document.getElementById('edCustomName').value = p.custom_name || '';

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

  const btn = document.getElementById('btnSaveProduct');
  btn.disabled = true;
  btn.textContent = 'Menyimpan...';

  const body = {
    is_active: document.getElementById('edIsActive').checked,
    custom_name: (document.getElementById('edCustomName').value || '').trim(),
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
    if (currentOrderFilter === 'HOLD' && o.fulfillment_status !== 'HOLD_PRICE_SPIKE') {
      return false;
    }
    const match = (o.invoice_number || '').toLowerCase().includes(search) ||
                  (o.customer_email || '').toLowerCase().includes(search) ||
                  (o.customer_contact || '').toLowerCase().includes(search) ||
                  (o.claim_code || '').toLowerCase().includes(search) ||
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
    } else if (o.fulfillment_status === 'REFUNDED') {
      fulfillLabel = '💸 Refunded';
      customFulfillStyle = 'background: rgba(148,163,184,0.15); color: #94a3b8; border: 1px solid rgba(148,163,184,0.3);';
    }

    const isFulfillFailed = o.payment_status === 'PAID' && o.fulfillment_status === 'FAILED';
    const isHoldSpike = o.fulfillment_status === 'HOLD_PRICE_SPIKE';

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
              ${isFulfillFailed ? `<button class="btn-retry" onclick="retryFulfillment('${escapeHtml(o.invoice_number)}')">Retry</button>` : ''}
              ${isHoldSpike ? `
                <button class="btn-action-mini" style="background: rgba(16,185,129,0.2); color: #34d399; border: 1px solid rgba(16,185,129,0.4);" onclick="forceFulfillOrder('${escapeHtml(o.invoice_number)}')">Beli</button>
                <button class="btn-action-mini" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" onclick="markRefundedOrder('${escapeHtml(o.invoice_number)}')">Refund</button>
              ` : ''}
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
            ${isFulfillFailed ? `<button class="btn-retry" onclick="retryFulfillment('${escapeHtml(o.invoice_number)}')">Retry Beli</button>` : ''}
            ${isHoldSpike ? `
              <button class="btn-action-mini" style="background: rgba(16,185,129,0.2); color: #34d399; border: 1px solid rgba(16,185,129,0.4);" onclick="forceFulfillOrder('${escapeHtml(o.invoice_number)}')">Beli Paksa</button>
              <button class="btn-action-mini" style="background: rgba(239,68,68,0.2); color: #f87171; border: 1px solid rgba(239,68,68,0.4);" onclick="markRefundedOrder('${escapeHtml(o.invoice_number)}')">Refund</button>
            ` : ''}
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
        ${items.length === 0 ? `<div style="color: var(--text-muted); font-size: 0.85rem;">Belum ada lisensi terkirim dari DigitalCore.</div>` : items.map(it => `
          <div class="license-item-box" style="margin-bottom: 6px;">
            <span>${escapeHtml(it)}</span>
          </div>
        `).join('')}
      </div>
    </div>
  `;

  modal?.classList.add('open');
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
    }
  } catch (err) {
    console.error('Failed to load settings:', err);
  }
}

async function saveSettings() {
  const btn = document.getElementById('btnSaveSettings');
  if (!btn) return;
  btn.disabled = true;
  btn.innerText = 'Menyimpan...';

  const body = {
    usd_rate: document.getElementById('cfgUsdRate').value,
    margin_percent: document.getElementById('cfgMarginPct').value,
    margin_fixed: document.getElementById('cfgMarginFixed').value,
    max_surge_disable_percent: document.getElementById('cfgMaxSurgePct')?.value || "50"
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
      showToast('Pengaturan kurs & margin berhasil disimpan! ✨');
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
// 5. INITIALIZATION & EVENT LISTENERS
// -------------------------------------------------------------
document.addEventListener('DOMContentLoaded', () => {
  loadStats();
  loadAdminProducts();
  loadOrders();
  loadSettings();

  // Refresh Button
  document.getElementById('btnRefreshStats')?.addEventListener('click', () => {
    loadStats();
    loadAdminProducts();
    loadOrders();
    loadSettings();
    showToast('Data diperbarui 🔄');
  });

  // Logout Button
  document.getElementById('btnLogout')?.addEventListener('click', async () => {
    localStorage.removeItem('storeku_admin_token');
    await fetch('/api/admin/logout', { method: 'POST' });
    window.location.href = '/admin/login';
  });

  // Settings Save
  document.getElementById('btnSaveSettings')?.addEventListener('click', saveSettings);

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

  // Close modals on backdrop click
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) {
      closeEditProductModal();
      document.getElementById('orderDetailModal')?.classList.remove('open');
    }
  });
});
