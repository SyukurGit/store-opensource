// Digihub License Claim Engine

function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2500);
}

function copyText(text) {
  navigator.clipboard.writeText(text).then(() => {
    showToast('Berhasil disalin! 📋');
  }).catch(() => {
    showToast('Gagal menyalin.');
  });
}

async function doClaim(queryCode) {
  const loading = document.getElementById('claimLoading');
  const errBox = document.getElementById('claimError');
  const resultBox = document.getElementById('claimResult');

  loading.style.display = 'block';
  errBox.style.display = 'none';
  resultBox.style.display = 'none';

  try {
    const res = await fetch(`/api/claim?code=${encodeURIComponent(queryCode)}`);
    const data = await res.json();

    loading.style.display = 'none';

    if (!res.ok || !data.success) {
      errBox.textContent = data.error || 'Pesanan dengan kode tersebut tidak ditemukan.';
      errBox.style.display = 'block';
      return;
    }

    renderClaimResult(data);
  } catch (err) {
    loading.style.display = 'none';
    errBox.textContent = 'Koneksi gagal: ' + err.message;
    errBox.style.display = 'block';
  }
}

function renderClaimResult(data) {
  const resultBox = document.getElementById('claimResult');
  resultBox.style.display = 'block';

  // Sync to local order history
  const orderHandler = window.DigihubOrders || window.StoreKuOrders;
  if (orderHandler && data.invoice_number) {
    orderHandler.saveOrder({
      invoice_number: data.invoice_number,
      claim_code: data.claim_code,
      product_name: data.product_name,
      quantity: data.quantity,
      payment_status: data.payment_status,
      created_at: data.created_at
    });
  }

  document.getElementById('resProductName').textContent = `${data.product_name} (${data.quantity} item)`;
  document.getElementById('resInvoice').textContent = `Invoice: ${data.invoice_number} | Kode Klaim: ${data.claim_code}`;

  const statusBadge = document.getElementById('resStatusBadge');
  if (data.payment_status === 'PAID') {
    statusBadge.className = 'badge badge-stock';
    statusBadge.textContent = '● PEMBAYARAN LUNAS';
  } else {
    statusBadge.className = 'badge badge-region';
    statusBadge.textContent = '● MENUNGGU PEMBAYARAN';
  }

  const itemsContainer = document.getElementById('resItemsContainer');
  const items = data.delivered_items || [];

  if (data.is_quota_out || data.fulfillment_status === 'HOLD_PRICE_SPIKE' || data.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE' || data.fulfillment_status === 'FAILED') {
    const isFailed = data.fulfillment_status === 'FAILED' || data.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE';
    const title = isFailed ? 'Kendala Pemenuhan Stok Upstream' : 'Kuota Produk Habis Terjual';
    const defaultMsg = isFailed
      ? "Pembayaran terverifikasi, namun terjadi kendala pemenuhan otomatis dari penyedia server (stok mendadak kosong/saldo bot). Dana Anda aman 100%! Silakan hubungi Customer Service kami via Telegram."
      : "Mohon maaf, kuota produk saat ini telah habis terjual. Dana pembayaran Anda aman 100%. Untuk proses pengembalian dana (refund), silakan hubungi Customer Service kami via Telegram.";

    statusBadge.className = 'badge badge-region';
    statusBadge.style.color = '#fbbf24';
    statusBadge.style.borderColor = 'rgba(245,158,11,0.4)';
    statusBadge.textContent = isFailed ? '● KENDALA STOK (DANA AMAN)' : '● DANA AMAN (KUOTA HABIS)';

    itemsContainer.innerHTML = `
      <div style="background: rgba(245, 158, 11, 0.08); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: 12px; padding: 20px; text-align: center;">
        <div style="font-size: 1.8rem; margin-bottom: 8px;">⚠️</div>
        <div style="font-weight: 700; color: #fbbf24; font-size: 1.05rem; margin-bottom: 8px;">${title}</div>
        <p style="color: var(--text-muted); font-size: 0.88rem; line-height: 1.6; margin-bottom: 16px;">
          ${escapeHtml(data.quota_out_message || defaultMsg)}
        </p>
        <a href="https://t.me/Digihubsupport" target="_blank" rel="noopener" class="btn btn-primary" style="display: inline-flex; align-items: center; gap: 8px; font-size: 0.88rem; padding: 10px 20px;" data-tele-cs>
          <span>Hubungi CS via Telegram</span> ➔
        </a>
      </div>
    `;
  } else if (items.length === 0) {
    if (data.payment_status === 'PAID') {
      itemsContainer.innerHTML = `
        <div style="background: rgba(245,158,11,0.1); padding: 12px; border-radius: 8px; color: #fbbf24; font-size: 0.85rem;">
          Pembayaran Anda telah diterima. Lisensi sedang diproses oleh antrean sistem dalam 1-2 menit.
        </div>
      `;
    } else {
      itemsContainer.innerHTML = `
        <div style="background: rgba(255,255,255,0.04); padding: 12px; border-radius: 8px; color: var(--text-muted); font-size: 0.85rem;">
          Pesanan ini belum dibayar. <a href="/payment?inv=${encodeURIComponent(data.invoice_number)}" style="color: var(--primary);">Klik di sini untuk bayar QRIS ➔</a>
        </div>
      `;
    }
  } else {
    itemsContainer.innerHTML = items.map(item => `
      <div class="license-item-box">
        <span>${escapeHtml(item)}</span>
        <button class="copy-mini-btn" onclick="copyText('${escapeHtml(item)}')">Salin</button>
      </div>
    `).join('');
  }

  document.getElementById('resWarrantyText').textContent = data.warranty_info || 'Garansi toko resmi aktif.';
  document.getElementById('resCreatedAt').textContent = data.created_at;

  const viewUrlWrap = document.getElementById('resViewUrlWrap');
  if (data.digitalcore_view_url) {
    viewUrlWrap.style.display = 'inline';
    document.getElementById('resViewUrl').href = data.digitalcore_view_url;
  } else {
    viewUrlWrap.style.display = 'none';
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>'"]/g, tag => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  }[tag] || tag));
}

document.addEventListener('DOMContentLoaded', () => {
  const params = new URLSearchParams(window.location.search);
  const codeParam = params.get('code') || params.get('invoice');

  if (codeParam) {
    document.getElementById('claimInput').value = codeParam;
    doClaim(codeParam);
  }

  document.getElementById('claimForm')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const q = document.getElementById('claimInput').value.trim();
    if (q) doClaim(q);
  });
});
