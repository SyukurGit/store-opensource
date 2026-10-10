// Digihub Unified Order History Engine
// Manages local order storage, real-time payment states, 15-min expiration audit & UI rendering

const DigihubOrders = (() => {
  const PRIMARY_STORAGE_KEY = 'digihub_orders';
  const LEGACY_STORAGE_KEYS = ['storeku_orders', 'storeku_history'];

  // Format currency helper
  function formatIDR(num) {
    return 'Rp ' + Number(num || 0).toLocaleString('id-ID');
  }

  // Safe escape HTML
  function escapeHtml(str) {
    if (!str) return '';
    return String(str).replace(/[&<>'"]/g, tag => ({
      '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
    }[tag] || tag));
  }

  // Check if expiration timestamp has elapsed
  function isExpired(expiresAt) {
    if (!expiresAt) return false;
    const expTime = new Date(expiresAt).getTime();
    if (isNaN(expTime)) return false;
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
        badge.textContent = list.length;
        badge.style.display = 'inline-block';
      } else {
        badge.style.display = 'none';
      }
    });
  }

  // Render modal list
  function renderHistoryList(containerId) {
    const container = document.getElementById(containerId);
    if (!container) return;

    const list = getOrders();

    if (list.length === 0) {
      container.innerHTML = `
        <div style="text-align: center; padding: 2.5rem 1rem; color: var(--text-muted);">
          <div style="font-size: 2.2rem; margin-bottom: 0.5rem; opacity: 0.7;">📦</div>
          <div style="font-weight: 600; color: #fff; margin-bottom: 0.25rem;">Belum Ada Riwayat Pesanan</div>
          <div style="font-size: 0.82rem;">Pesanan yang Anda buat akan otomatis tercatat di browser ini tanpa perlu registrasi akun.</div>
        </div>
      `;
      return;
    }

    container.innerHTML = list.map(item => {
      const inv = escapeHtml(item.invoice_number);
      const claim = escapeHtml(item.claim_code);
      const name = escapeHtml(item.product_name);
      const qty = item.quantity || 1;
      const amountStr = formatIDR(item.total_amount);

      let effectiveStatus = item.payment_status || 'PENDING';
      const expiredByTime = effectiveStatus === 'PENDING' && isExpired(item.expires_at);

      if (effectiveStatus === 'PAID') {
        // Paid item
        return `
          <div class="order-history-card active-card" onclick="window.location.href='/claim?code=${encodeURIComponent(item.claim_code || item.invoice_number)}'">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title">${name}</span>
                <span class="badge-history badge-history-paid">● Lunas</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <strong style="color: #10b981;">${amountStr}</strong>
              </div>
              <div class="order-card-sub" style="color: #a855f7;">
                Kode Klaim: <code style="color: #c084fc; font-weight: 700;">${claim || '-'}</code>
              </div>
            </div>
            <div class="order-card-action">
              <a href="/claim?code=${encodeURIComponent(item.claim_code || item.invoice_number)}" class="btn-history-action btn-history-claim" onclick="event.stopPropagation();">
                Buka Lisensi 🔑
              </a>
            </div>
          </div>
        `;
      } else if (effectiveStatus === 'CANCELLED') {
        // Cancelled order (DISABLED, non-clickable)
        return `
          <div class="order-history-card order-card-disabled">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title" style="color: #9ca3af;">${name}</span>
                <span class="badge-history badge-history-cancelled">✕ Order Dibatalkan</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <span>${amountStr}</span>
              </div>
              <div class="order-card-sub" style="color: #6b7280;">
                Pesanan telah dibatalkan oleh pembeli
              </div>
            </div>
            <div class="order-card-action">
              <span class="btn-history-disabled">Dibatalkan</span>
            </div>
          </div>
        `;
      } else if (effectiveStatus === 'EXPIRED' || expiredByTime) {
        // Expired order (DISABLED, non-clickable)
        return `
          <div class="order-history-card order-card-disabled">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title" style="color: #9ca3af;">${name}</span>
                <span class="badge-history badge-history-expired">⌛ Kadaluarsa</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <span>${amountStr}</span>
              </div>
              <div class="order-card-sub" style="color: #6b7280;">
                Batas waktu bayar 15 menit telah habis
              </div>
            </div>
            <div class="order-card-action">
              <span class="btn-history-disabled">Kadaluarsa</span>
            </div>
          </div>
        `;
      } else {
        // Unpaid pending order (CLICKABLE to pay QRIS)
        return `
          <div class="order-history-card active-card highlight-unpaid" onclick="window.location.href='/payment?inv=${encodeURIComponent(item.invoice_number)}'">
            <div class="order-card-main">
              <div class="order-card-header">
                <span class="order-card-title">${name}</span>
                <span class="badge-history badge-history-unpaid">⏳ Belum Bayar</span>
              </div>
              <div class="order-card-meta">
                <span>${inv}</span>
                <span>•</span>
                <span>Qty: ${qty}</span>
                <span>•</span>
                <strong style="color: #fbbf24;">${amountStr}</strong>
              </div>
              <div class="order-card-sub" style="color: #fbbf24;">
                Klik untuk scan & bayar QRIS sekarang ➔
              </div>
            </div>
            <div class="order-card-action">
              <a href="/payment?inv=${encodeURIComponent(item.invoice_number)}" class="btn-history-action btn-history-pay" onclick="event.stopPropagation();">
                Bayar QRIS ➔
              </a>
            </div>
          </div>
        `;
      }
    }).join('');
  }

  // Public Interface
  return {
    getOrders,
    saveOrder,
    updateOrderStatus,
    isExpired,
    updateBadge,
    renderHistoryList
  };
})();

// Export globally with backwards-compatible alias
window.DigihubOrders = DigihubOrders;
window.StoreKuOrders = DigihubOrders;

// Auto-init badge on DOM ready
document.addEventListener('DOMContentLoaded', () => {
  DigihubOrders.updateBadge();
});

// Auto-sync Telegram support username dynamically from /api/config
(async () => {
  try {
    let username = sessionStorage.getItem('digihub_tele_user');
    if (!username) {
      const res = await fetch('/api/config');
      if (res.ok) {
        const data = await res.json();
        if (data.success && data.telegram_username) {
          username = data.telegram_username;
          sessionStorage.setItem('digihub_tele_user', username);
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
