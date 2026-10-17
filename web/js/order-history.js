// Digimest Unified Order History Engine
// Manages local order storage, real-time payment states, 15-min expiration audit & UI rendering

const DigimestOrders = (() => {
  const PRIMARY_STORAGE_KEY = 'digimest_orders';
  const LEGACY_STORAGE_KEYS = ['digihub_orders', 'diginest_orders', 'storeku_orders', 'storeku_history'];

  // Format currency helper
  function formatIDR(num) {
    if (window.DigimestCurrency && typeof window.DigimestCurrency.format === 'function') {
      return window.DigimestCurrency.format(num);
    }
    return 'Rp ' + Number(num || 0).toLocaleString('id-ID');
  }

  // Safe escape HTML
  function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/[&<>'"]/g, tag => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
    }[tag] || tag));
  }

  // Safe timestamp parser (normalizes strings without timezone as UTC)
  function parseTimestamp(dateStr) {
    if (!dateStr) return null;
    if (typeof dateStr === 'number') return isNaN(dateStr) ? null : dateStr;
    let clean = String(dateStr).trim();
    if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/.test(clean)) {
      clean = clean.replace(' ', 'T') + 'Z';
    } else if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?$/.test(clean)) {
      clean = clean + 'Z';
    }
    const t = new Date(clean).getTime();
    return isNaN(t) ? null : t;
  }

  // Check if order or expiration timestamp has elapsed
  function isExpired(target) {
    if (!target) return false;

    // If passed an order object
    if (typeof target === 'object') {
      if (target.payment_status === 'EXPIRED') return true;

      if (target.expires_at) {
        const expTime = parseTimestamp(target.expires_at);
        if (expTime) return Date.now() > expTime;
      }

      // Fallback: created_at + 15 minutes
      if (target.created_at) {
        const createTime = parseTimestamp(target.created_at);
        if (createTime) return Date.now() > (createTime + 15 * 60 * 1000);
      }

      return false;
    }

    // If passed a timestamp or date string directly
    const expTime = parseTimestamp(target);
    if (!expTime) return false;
    return Date.now() > expTime;
  }

  // Retrieve and auto-migrate legacy history
  function getOrders() {
    try {
      let orders = JSON.parse(localStorage.getItem(PRIMARY_STORAGE_KEY) || '[]');
      
      // Auto-migrate from legacy keys if primary is empty or legacy has unmerged items
      let merged = false;
      const existingInv = new Set(orders.map(o => o.invoice_number));

      for (const legKey of LEGACY_STORAGE_KEYS) {
        const legacy = JSON.parse(localStorage.getItem(legKey) || '[]');
        if (Array.isArray(legacy) && legacy.length > 0) {
          for (const item of legacy) {
            if (item && item.invoice_number && !existingInv.has(item.invoice_number)) {
              orders.push({
                invoice_number: item.invoice_number,
                claim_code: item.claim_code || '',
                product_name: item.product_name || 'Produk Digital',
                quantity: item.quantity || 1,
                total_amount: item.total_amount || 0,
                payment_status: item.payment_status || 'PENDING',
                expires_at: item.expires_at || null,
                created_at: item.created_at || new Date().toISOString()
              });
              existingInv.add(item.invoice_number);
              merged = true;
            }
          }
        }
      }

      if (merged) {
        localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(orders));
      }

      return Array.isArray(orders) ? orders : [];
    } catch (e) {
      console.warn('DigihubOrders: Failed to read orders from localStorage:', e);
      return [];
    }
  }

  // Save or update an order in storage
  function saveOrder(orderData) {
    if (!orderData || !orderData.invoice_number) return;

    try {
      let list = getOrders();
      const existingIdx = list.findIndex(o => o.invoice_number === orderData.invoice_number);

      if (existingIdx >= 0) {
        // Merge existing fields to avoid losing claim_code or product details
        list[existingIdx] = {
          ...list[existingIdx],
          ...orderData,
          claim_code: orderData.claim_code || list[existingIdx].claim_code || '',
          payment_status: orderData.payment_status || list[existingIdx].payment_status || 'PENDING',
          expires_at: orderData.expires_at || list[existingIdx].expires_at || null
        };
      } else {
        // New order pushed to front
        list.unshift({
          invoice_number: orderData.invoice_number,
          claim_code: orderData.claim_code || '',
          product_name: orderData.product_name || 'Produk Digital',
          quantity: orderData.quantity || 1,
          total_amount: orderData.total_amount || 0,
          payment_status: orderData.payment_status || 'PENDING',
          expires_at: orderData.expires_at || null,
          created_at: orderData.created_at || new Date().toISOString()
        });
      }

      if (list.length > 50) list = list.slice(0, 50);
      localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
      localStorage.setItem('storeku_orders', JSON.stringify(list));
      updateBadge();
    } catch (e) {
      console.warn('DigihubOrders: Failed to save order:', e);
    }
  }

  // Update specific order status
  function updateOrderStatus(invoiceNumber, status) {
    if (!invoiceNumber) return;
    try {
      let list = getOrders();
      let changed = false;
      for (let item of list) {
        if (item.invoice_number === invoiceNumber) {
          item.payment_status = status;
          changed = true;
        }
      }
      if (changed) {
        localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
        localStorage.setItem('storeku_orders', JSON.stringify(list));
      }
    } catch (e) {
      console.warn('DigihubOrders: Failed to update order status:', e);
    }
  }

  // Update navbar badge count
  function updateBadge() {
    const list = getOrders();
    const badges = document.querySelectorAll('#historyBadge, .history-badge-count');
    badges.forEach(badge => {
      if (list.length > 0) {
        if (badge.classList.contains('hamburger-badge-dot')) {
          badge.style.display = 'block';
        } else {
          badge.textContent = list.length;
          badge.style.display = 'inline-block';
        }
      } else {
        badge.style.display = 'none';
      }
    });
  }

  // Remove a single order from history
  function removeOrder(invoiceNumber, containerId) {
    if (!invoiceNumber) return;
    try {
      let list = getOrders();
      list = list.filter(o => o.invoice_number !== invoiceNumber);
      localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
      for (const legKey of LEGACY_STORAGE_KEYS) {
        try {
          let legacy = JSON.parse(localStorage.getItem(legKey) || '[]');
          if (Array.isArray(legacy)) {
            legacy = legacy.filter(o => o.invoice_number !== invoiceNumber);
            localStorage.setItem(legKey, JSON.stringify(legacy));
          }
        } catch (e) {}
      }
      updateBadge();
      if (containerId) {
        renderHistoryList(containerId, true);
      }
    } catch (e) {
      console.warn('DigimestOrders: Failed to remove order:', e);
    }
  }

  // Clear all expired or cancelled orders from history
  function clearExpiredOrders(containerId) {
    try {
      let list = getOrders();
      list = list.filter(o => {
        if (o.payment_status === 'EXPIRED' || o.payment_status === 'CANCELLED') return false;
        if (o.payment_status === 'PENDING' && isExpired(o)) return false;
        return true;
      });
      localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
      for (const legKey of LEGACY_STORAGE_KEYS) {
        try {
          let legacy = JSON.parse(localStorage.getItem(legKey) || '[]');
          if (Array.isArray(legacy)) {
            legacy = legacy.filter(o => {
              if (o.payment_status === 'EXPIRED' || o.payment_status === 'CANCELLED') return false;
              if (o.payment_status === 'PENDING' && isExpired(o)) return false;
              return true;
            });
            localStorage.setItem(legKey, JSON.stringify(legacy));
          }
        } catch (e) {}
      }
      updateBadge();
      if (containerId) {
        renderHistoryList(containerId, true);
      }
    } catch (e) {
      console.warn('DigimestOrders: Failed to clear expired orders:', e);
    }
  }

  // Background sync for pending orders against backend API
  let isSyncing = false;
  let lastSyncTime = 0;

  async function syncPendingOrders(containerId) {
    const now = Date.now();
    if (isSyncing || (now - lastSyncTime < 8000)) return;
    isSyncing = true;
    lastSyncTime = now;

    try {
      let list = getOrders();
      const pendingOrders = list.filter(o => !o.payment_status || o.payment_status === 'PENDING');
      if (pendingOrders.length === 0) {
        isSyncing = false;
        return;
      }

      let changed = false;
      await Promise.all(pendingOrders.slice(0, 10).map(async (order) => {
        try {
          const inv = encodeURIComponent(order.invoice_number);
          const statusUrl = window.digimestApi ? window.digimestApi(`/api/orders/${inv}/status`) : `/api/orders/${inv}/status`;
          const res = await fetch(statusUrl);
          if (!res.ok) return;
          const json = await res.json();
          if (json.success && json.data) {
            const s = json.data.payment_status;
            if (s && s !== order.payment_status) {
              order.payment_status = s;
              changed = true;
            }
            if (json.data.claim_code && json.data.claim_code !== order.claim_code) {
              order.claim_code = json.data.claim_code;
              changed = true;
            }
            if (json.data.expires_at && json.data.expires_at !== order.expires_at) {
              order.expires_at = json.data.expires_at;
              changed = true;
            }
          }
        } catch (err) {}
      }));

      if (changed) {
        localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
        localStorage.setItem('storeku_orders', JSON.stringify(list));
        updateBadge();
        if (containerId) {
          renderHistoryList(containerId, true);
        }
      }
    } catch (e) {
      console.warn('DigimestOrders: Background sync failed:', e);
    } finally {
      isSyncing = false;
    }
  }

  // Render modal list
  function renderHistoryList(containerId = 'historyListContainer', skipSync = false) {
    const targetId = containerId || 'historyListContainer';
    const container = document.getElementById(targetId);
    if (!container) return;

    const isEn = (typeof document !== 'undefined' && document.documentElement && document.documentElement.lang === 'en') ||
                 (typeof window !== 'undefined' && window.location && window.location.pathname.startsWith('/en'));

    let list = getOrders();

    // Local audit: immediately flag pending items that have expired by time
    let hasLocalChanges = false;
    for (const item of list) {
      if ((!item.payment_status || item.payment_status === 'PENDING') && isExpired(item)) {
        item.payment_status = 'EXPIRED';
        hasLocalChanges = true;
      }
    }
    if (hasLocalChanges) {
      localStorage.setItem(PRIMARY_STORAGE_KEY, JSON.stringify(list));
      localStorage.setItem('storeku_orders', JSON.stringify(list));
    }

    if (list.length === 0) {
      const emptyTitle = isEn ? 'No Order History Yet' : 'Belum Ada Riwayat Pesanan';
      const emptyDesc = isEn 
        ? 'Orders you place will be automatically recorded in this browser without needing an account.'
        : 'Pesanan yang Anda buat akan otomatis tercatat di browser ini tanpa perlu registrasi akun.';
      const exploreBtnText = isEn ? 'Explore Store Catalog ➔' : 'Jelajahi Katalog Toko ➔';
      const catalogHref = isEn ? '/en/' : '/';

      container.innerHTML = `
        <div style="text-align: center; padding: 2.5rem 1rem; color: var(--text-muted);">
          <div style="font-size: 2.2rem; margin-bottom: 0.5rem; opacity: 0.7;">📦</div>
          <div style="font-weight: 600; color: #fff; margin-bottom: 0.25rem;">${emptyTitle}</div>
          <div style="font-size: 0.82rem; margin-bottom: 1.25rem;">${emptyDesc}</div>
          <a href="${catalogHref}" class="btn-buy" style="display: inline-block; padding: 0.55rem 1.4rem; font-size: 0.85rem; text-decoration: none;" onclick="document.getElementById('historyModal')?.classList.remove('open'); document.body.style.overflow = '';">
            ${exploreBtnText}
          </a>
        </div>
      `;
      return;
    }

    // Check if there are any expired or cancelled orders to show clear banner
    const hasExpiredOrCancelled = list.some(item => item.payment_status === 'EXPIRED' || item.payment_status === 'CANCELLED' || isExpired(item));
    
    let html = '';
    if (hasExpiredOrCancelled) {
      const bannerText = isEn ? '⚠️ Expired / cancelled orders found' : '⚠️ Ditemukan pesanan kadaluarsa / dibatalkan';
      const clearBtnText = isEn ? 'Clear Expired Orders' : 'Bersihkan Riwayat Kadaluarsa';
      html += `
        <div class="history-clear-bar">
          <span>${bannerText}</span>
          <button type="button" class="btn-clear-expired" onclick="DigimestOrders.clearExpiredOrders('${escapeHtml(targetId)}')">${clearBtnText}</button>
        </div>
      `;
    }

    html += list.map(item => {
      const inv = escapeHtml(item.invoice_number);
      const claim = escapeHtml(item.claim_code);
      const name = escapeHtml(item.product_name);
      const qty = item.quantity || 1;
      const amountStr = formatIDR(item.total_amount);

      let effectiveStatus = item.payment_status || 'PENDING';
      const expiredByTime = effectiveStatus === 'PENDING' && isExpired(item);

      if (effectiveStatus === 'PAID') {
        // Paid item (clickable to claim)
        const claimUrl = isEn ? `/en/claim?code=${encodeURIComponent(item.claim_code || item.invoice_number)}` : `/claim?code=${encodeURIComponent(item.claim_code || item.invoice_number)}`;
        const paidBadge = isEn ? '● Paid' : '● Lunas';
        const claimLabel = isEn ? 'Claim Code:' : 'Kode Klaim:';
        const claimBtn = isEn ? 'View License 🔑' : 'Buka Lisensi 🔑';

        return `
          <div class="order-history-card active-card" onclick="window.location.href='${claimUrl}'">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title">${name}</span>
                <span class="badge-history badge-history-paid">${paidBadge}</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <strong style="color: #10b981;">${amountStr}</strong>
              </div>
              <div class="order-card-sub" style="color: #a855f7;">
                ${claimLabel} <code style="color: #c084fc; font-weight: 700;">${claim || '-'}</code>
              </div>
            </div>
            <div class="order-card-action">
              <a href="${claimUrl}" class="btn-history-action btn-history-claim" onclick="event.stopPropagation();">
                ${claimBtn}
              </a>
            </div>
          </div>
        `;
      } else if (effectiveStatus === 'CANCELLED') {
        // Cancelled order (DISABLED, 100% non-clickable with delete button)
        const cancelledBadge = isEn ? '✕ Cancelled' : '✕ Dibatalkan';
        const cancelledSub = isEn ? 'Order was cancelled by buyer' : 'Pesanan telah dibatalkan oleh pembeli';
        const deleteTitle = isEn ? 'Remove from history' : 'Hapus dari riwayat';

        return `
          <div class="order-history-card order-card-disabled">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title" style="color: #9ca3af;">${name}</span>
                <span class="badge-history badge-history-cancelled">${cancelledBadge}</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <span>${amountStr}</span>
              </div>
              <div class="order-card-sub" style="color: #6b7280;">
                ${cancelledSub}
              </div>
            </div>
            <div class="order-card-action">
              <span class="btn-history-disabled">${cancelledBadge}</span>
              <button type="button" class="btn-history-delete" title="${deleteTitle}" onclick="event.stopPropagation(); DigimestOrders.removeOrder('${inv}', '${escapeHtml(targetId)}');">✕</button>
            </div>
          </div>
        `;
      } else if (effectiveStatus === 'EXPIRED' || expiredByTime) {
        // Expired order (DISABLED, 100% non-clickable with delete button)
        const expiredBadge = isEn ? '⌛ Expired' : '⌛ Kadaluarsa';
        const expiredSub = isEn ? '15-minute payment window has expired' : 'Batas waktu bayar 15 menit telah habis';
        const deleteTitle = isEn ? 'Remove from history' : 'Hapus dari riwayat';

        return `
          <div class="order-history-card order-card-disabled">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title" style="color: #9ca3af;">${name}</span>
                <span class="badge-history badge-history-expired">${expiredBadge}</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <span>${amountStr}</span>
              </div>
              <div class="order-card-sub" style="color: #6b7280;">
                ${expiredSub}
              </div>
            </div>
            <div class="order-card-action">
              <span class="btn-history-disabled">${expiredBadge}</span>
              <button type="button" class="btn-history-delete" title="${deleteTitle}" onclick="event.stopPropagation(); DigimestOrders.removeOrder('${inv}', '${escapeHtml(targetId)}');">✕</button>
            </div>
          </div>
        `;
      } else {
        // Unpaid pending order (CLICKABLE to pay QRIS)
        const payUrl = isEn ? `/en/payment?inv=${encodeURIComponent(item.invoice_number)}` : `/payment?inv=${encodeURIComponent(item.invoice_number)}`;
        const pendingBadge = isEn ? '⏳ Unpaid' : '⏳ Belum Bayar';
        const pendingSub = isEn ? 'Click to scan & pay QRIS now ➔' : 'Klik untuk scan & bayar QRIS sekarang ➔';
        const payBtn = isEn ? 'Pay QRIS ➔' : 'Bayar QRIS ➔';

        return `
          <div class="order-history-card active-card highlight-unpaid" onclick="window.location.href='${payUrl}'">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title">${name}</span>
                <span class="badge-history badge-history-unpaid">${pendingBadge}</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <strong style="color: #fbbf24;">${amountStr}</strong>
              </div>
              <div class="order-card-sub" style="color: #fbbf24;">
                ${pendingSub}
              </div>
            </div>
            <div class="order-card-action">
              <a href="${payUrl}" class="btn-history-action btn-history-pay" onclick="event.stopPropagation();">
                ${payBtn}
              </a>
            </div>
          </div>
        `;
      }
    }).join('');

    container.innerHTML = html;

    // Trigger background sync if not skipped
    if (!skipSync) {
      syncPendingOrders(targetId);
    }
  }

  // Unified modal openers & closers
  function openHistoryModal(containerId = 'historyListContainer', modalId = 'historyModal') {
    const targetContainerId = containerId || 'historyListContainer';
    const targetModalId = modalId || 'historyModal';
    renderHistoryList(targetContainerId);
    const modal = document.getElementById(targetModalId);
    if (modal) {
      modal.classList.add('open');
      document.body.style.overflow = 'hidden';
    }
  }

  function closeHistoryModal(modalId = 'historyModal') {
    const targetModalId = modalId || 'historyModal';
    const modal = document.getElementById(targetModalId);
    if (modal) {
      modal.classList.remove('open');
      document.body.style.overflow = '';
    }
  }

  // Public Interface
  return {
    getOrders,
    saveOrder,
    updateOrderStatus,
    isExpired,
    removeOrder,
    clearExpiredOrders,
    syncPendingOrders,
    updateBadge,
    renderHistoryList,
    renderHistoryUI: (containerId = 'historyListContainer') => renderHistoryList(containerId || 'historyListContainer'),
    openModal: openHistoryModal,
    closeModal: closeHistoryModal
  };
})();

// Export globally with backwards-compatible aliases
window.DigimestOrders = DigimestOrders;
window.DigihubOrders = DigimestOrders;
window.StoreKuOrders = DigimestOrders;

// Auto-init badge on DOM ready
document.addEventListener('DOMContentLoaded', () => {
  DigimestOrders.updateBadge();
});

// Auto-sync Telegram support username dynamically from /api/config
(async () => {
  try {
    let username = sessionStorage.getItem('digimest_tele_user') || sessionStorage.getItem('digihub_tele_user');
    if (!username) {
      const configUrl = window.digimestApi ? window.digimestApi('/api/config') : '/api/config';
      const res = await fetch(configUrl);
      if (res.ok) {
        const data = await res.json();
        if (data.success && data.telegram_username) {
          username = data.telegram_username;
          sessionStorage.setItem('digimest_tele_user', username);
        }
      }
    }
    if (username) {
      document.querySelectorAll('a[href*="t.me/"]').forEach(a => {
        a.href = a.href.replace(/t\.me\/[a-zA-Z0-9_]+/, `t.me/${username}`);
      });
      document.querySelectorAll('[data-tele-cs]').forEach(el => {
        if (el.textContent.includes('Bantuan CS Telegram')) {
          el.textContent = `💬 Bantuan CS Telegram: @${username}`;
        } else if (el.textContent.includes('Chat CS Telegram')) {
          el.textContent = `💬 Chat CS Telegram: @${username}`;
        }
      });
    }
  } catch (e) {}
})();

// ==============================================================
// DIGIMEST CURRENCY MANAGER (USD & IDR SMART SWITCHER)
// ==============================================================
window.DigimestCurrency = {
  getCurrency: function() {
    return localStorage.getItem('digimest_currency') || 'IDR';
  },
  getRate: function() {
    return window.digimestUSDRate || parseFloat(sessionStorage.getItem('digimest_usd_rate')) || 18000;
  },
  setCurrency: function(curr, isManual = true) {
    const cur = (curr === 'USD') ? 'USD' : 'IDR';
    try {
      localStorage.setItem('digimest_currency', cur);
      if (isManual) {
        localStorage.setItem('digimest_currency_manual', '1');
      }
    } catch (e) {}
    this.syncUI();
    window.dispatchEvent(new CustomEvent('currencychange', { detail: { currency: cur } }));
  },
  format: function(amountIDR, amountUSD = 0) {
    const cur = this.getCurrency();
    if (cur === 'USD') {
      let usdVal = Number(amountUSD);
      if (!usdVal || usdVal <= 0) {
        const rate = this.getRate();
        usdVal = Number(amountIDR || 0) / rate;
      }
      return '$' + usdVal.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    }
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0
    }).format(Number(amountIDR || 0));
  },
  formatEstimate: function(amountIDR, amountUSD = 0) {
    const cur = this.getCurrency();
    const rate = this.getRate();
    if (cur === 'USD') {
      const idrVal = Number(amountIDR || 0);
      return `~Rp ${idrVal.toLocaleString('id-ID')}`;
    } else {
      const usdVal = amountUSD > 0 ? amountUSD : (Number(amountIDR || 0) / rate);
      return `~$${usdVal.toFixed(2)}`;
    }
  },
  syncUI: function() {
    const cur = this.getCurrency();
    const isUSD = cur === 'USD';

    // Update Desktop Navbar Button
    const desktopCode = document.getElementById('desktopCurrencyCode');
    const desktopSymbol = document.querySelector('.currency-symbol-icon');
    if (desktopCode) desktopCode.textContent = cur;
    if (desktopSymbol) desktopSymbol.textContent = isUSD ? '$' : 'Rp';

    // Update Desktop Menu items checkmarks & active class
    document.querySelectorAll('.currency-item').forEach(el => {
      const itemCur = el.getAttribute('data-currency');
      const check = el.querySelector('.currency-check');
      const isActive = itemCur === cur;
      el.classList.toggle('active', isActive);
      if (check) check.style.visibility = isActive ? 'visible' : 'hidden';
    });

    // Update Mobile Drawer items
    document.querySelectorAll('.drawer-currency-btn').forEach(btn => {
      const btnCur = btn.getAttribute('data-currency');
      const check = btn.querySelector('.currency-check');
      const isActive = btnCur === cur;
      btn.classList.toggle('active', isActive);
      if (check) check.style.visibility = isActive ? 'visible' : 'hidden';
    });
  }
};

// Listen to app config loaded to update rate if changed
window.addEventListener('appconfigloaded', () => {
  if (window.DigimestCurrency) {
    window.DigimestCurrency.syncUI();
  }
});

// Mobile Navigation Drawer, Language & Currency Switcher Manager
(() => {
  function initMobileNav() {
    const hamburgerBtn = document.getElementById('navHamburgerBtn');
    const drawerOverlay = document.getElementById('mobileDrawerOverlay');
    const drawerCloseBtn = document.getElementById('drawerCloseBtn');
    const drawerHistoryBtn = document.getElementById('drawerOpenHistoryBtn');
    const langBtn = document.getElementById('langDropdownBtn');
    const langWrap = langBtn?.closest('.lang-dropdown-wrap');
    const curBtn = document.getElementById('currencyDropdownBtn');
    const curWrap = curBtn?.closest('.currency-dropdown-wrap');

    function openDrawer() {
      hamburgerBtn?.classList.add('open');
      drawerOverlay?.classList.add('open');
      document.body.style.overflow = 'hidden';
      // Sync UI on drawer open
      window.DigimestCurrency.syncUI();
    }

    function closeDrawer() {
      hamburgerBtn?.classList.remove('open');
      drawerOverlay?.classList.remove('open');
      document.body.style.overflow = '';
    }

    hamburgerBtn?.addEventListener('click', (e) => {
      e.stopPropagation();
      if (drawerOverlay?.classList.contains('open')) {
        closeDrawer();
      } else {
        openDrawer();
      }
    });

    drawerCloseBtn?.addEventListener('click', closeDrawer);

    drawerOverlay?.addEventListener('click', (e) => {
      if (e.target === drawerOverlay) {
        closeDrawer();
      }
    });

    // Language Dropdown Toggle
    langBtn?.addEventListener('click', (e) => {
      e.stopPropagation();
      curWrap?.classList.remove('open');
      langWrap?.classList.toggle('open');
    });

    // Currency Dropdown Toggle (Desktop)
    curBtn?.addEventListener('click', (e) => {
      e.stopPropagation();
      langWrap?.classList.remove('open');
      curWrap?.classList.toggle('open');
    });

    document.addEventListener('click', (e) => {
      if (langWrap && !langWrap.contains(e.target)) {
        langWrap.classList.remove('open');
      }
      if (curWrap && !curWrap.contains(e.target)) {
        curWrap.classList.remove('open');
      }
    });

    // Desktop Currency Switcher items click
    document.querySelectorAll('.currency-item').forEach(el => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        const targetCur = el.getAttribute('data-currency');
        curWrap?.classList.remove('open');
        window.DigimestCurrency.setCurrency(targetCur, true);
      });
    });

    // Mobile Drawer Currency items click
    document.querySelectorAll('.drawer-currency-btn').forEach(el => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        const targetCur = el.getAttribute('data-currency');
        window.DigimestCurrency.setCurrency(targetCur, true);
      });
    });

    // Sync Currency UI
    window.DigimestCurrency.syncUI();

    // Sync Language Switcher UI with current page language
    const isEnPage = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

    // Update navbar button label & flag if English
    if (isEnPage && langBtn) {
      langBtn.innerHTML = `
        <svg class="svg-flag" viewBox="0 0 60 40" aria-hidden="true"><rect width="60" height="40" fill="#012169"/><path d="M0 0L60 40M60 0L0 40" stroke="#ffffff" stroke-width="8"/><path d="M0 0L60 40M60 0L0 40" stroke="#c8102e" stroke-width="3.5"/><path d="M30 0V40M0 20H60" stroke="#ffffff" stroke-width="12"/><path d="M30 0V40M0 20H60" stroke="#c8102e" stroke-width="7"/></svg>
        <span class="lang-code">EN</span>
        <span class="lang-arrow">▾</span>
      `;
    }

    // Update active checkmarks in menu & drawer
    document.querySelectorAll('[data-lang]').forEach(el => {
      const lang = el.getAttribute('data-lang');
      const check = el.querySelector('.lang-check');
      if (lang === 'en') {
        if (isEnPage) {
          el.classList.add('active');
          if (check) check.style.visibility = 'visible';
        } else {
          el.classList.remove('active');
          if (check) check.style.visibility = 'hidden';
        }
      } else if (lang === 'id') {
        if (!isEnPage) {
          el.classList.add('active');
          if (check) check.style.visibility = 'visible';
        } else {
          el.classList.remove('active');
          if (check) check.style.visibility = 'hidden';
        }
      }
    });

    // Automatic Link Enforcer for English pages: ensures all internal links stay within /en/
    if (isEnPage) {
      document.querySelectorAll('a[href]').forEach(a => {
        let href = a.getAttribute('href');
        if (!href || href.startsWith('#') || href.startsWith('http') || href.startsWith('//') || href.startsWith('javascript:') || href.startsWith('mailto:') || href.startsWith('tel:')) return;
        if (href.startsWith('/admin') || href.startsWith('/api') || href.startsWith('/en/') || href.startsWith('/css/') || href.startsWith('/js/') || href.startsWith('/logo/')) return;
        
        if (href === '/' || href === '/index.html') {
          a.setAttribute('href', '/en/');
        } else if (href.startsWith('/')) {
          a.setAttribute('href', '/en' + href);
        }
      });
    }

    // Language item click with intelligent path mapping & smart currency transition
    document.querySelectorAll('[data-lang]').forEach(el => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        const targetLang = el.getAttribute('data-lang');
        const currentPath = window.location.pathname;
        const currentSearch = window.location.search;
        const currentHash = window.location.hash;

        closeDrawer();
        langWrap?.classList.remove('open');

        // Extract clean route (e.g. '/product', '/en/how-to-buy.html' -> 'how-to-buy')
        let cleanRoute = currentPath.replace(/^\/+/, '').replace(/\.html$/, '');
        let isEnCurrent = cleanRoute === 'en' || cleanRoute.startsWith('en/');
        let baseRoute = isEnCurrent ? cleanRoute.replace(/^en\/?/, '') : cleanRoute;

        if (targetLang === 'en') {
          try {
            localStorage.setItem('digimest_lang', 'en');
            document.cookie = "digimest_lang=en; path=/; max-age=31536000; SameSite=Lax";
            // SMART CURRENCY TRANSITION:
            // If user has not manually locked currency, auto switch to USD
            if (!localStorage.getItem('digimest_currency_manual')) {
              localStorage.setItem('digimest_currency', 'USD');
            }
          } catch (err) {}
          if (isEnCurrent) return;
          let targetPath = '/en/';
          if (baseRoute && baseRoute !== 'index') {
            targetPath = '/en/' + baseRoute + currentSearch + currentHash;
          }
          window.location.href = targetPath;
        } else if (targetLang === 'id') {
          try {
            localStorage.setItem('digimest_lang', 'id');
            document.cookie = "digimest_lang=id; path=/; max-age=31536000; SameSite=Lax";
            // SMART CURRENCY TRANSITION:
            // If user has not manually locked currency, auto switch to IDR
            if (!localStorage.getItem('digimest_currency_manual')) {
              localStorage.setItem('digimest_currency', 'IDR');
            }
          } catch (err) {}
          if (!isEnCurrent) return;
          let targetPath = '/';
          if (baseRoute && baseRoute !== 'index') {
            targetPath = '/' + baseRoute + currentSearch + currentHash;
          }
          window.location.href = targetPath;
        }
      });
    });

    // When drawer history button is clicked, close drawer and open history modal
    drawerHistoryBtn?.addEventListener('click', (e) => {
      e.preventDefault();
      closeDrawer();
      setTimeout(() => {
        if (window.DigimestOrders && typeof window.DigimestOrders.openModal === 'function') {
          window.DigimestOrders.openModal('historyListContainer', 'historyModal');
        } else if (typeof openHistoryModal === 'function') {
          openHistoryModal('historyListContainer', 'historyModal');
        }
      }, 150);
    });

    // Central modal close listeners
    document.getElementById('closeHistoryBtn')?.addEventListener('click', () => {
      if (window.DigimestOrders && typeof window.DigimestOrders.closeModal === 'function') {
        window.DigimestOrders.closeModal('historyModal');
      } else if (typeof closeHistoryModal === 'function') {
        closeHistoryModal('historyModal');
      }
    });

    document.getElementById('historyModal')?.addEventListener('click', (e) => {
      if (e.target.id === 'historyModal') {
        if (window.DigimestOrders && typeof window.DigimestOrders.closeModal === 'function') {
          window.DigimestOrders.closeModal('historyModal');
        } else if (typeof closeHistoryModal === 'function') {
          closeHistoryModal('historyModal');
        }
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initMobileNav);
  } else {
    initMobileNav();
  }
})();
