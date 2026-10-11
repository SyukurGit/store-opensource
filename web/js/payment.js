// Digihub Payment Page Engine

let pollInterval = null;
let timerInterval = null;
let currentInvoice = '';
let currentClaimCode = '';
let deliveredKeys = [];

function formatIDR(num) {
  return 'Rp ' + Number(num || 0).toLocaleString('id-ID');
}

function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2500);
}

function copyText(text) {
  navigator.clipboard.writeText(text).then(() => {
    showToast('Berhasil disalin ke clipboard! 📋');
  }).catch(() => {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    showToast('Berhasil disalin!');
  });
}

async function loadPaymentDetails() {
  const params = new URLSearchParams(window.location.search);
  currentInvoice = params.get('inv') || params.get('invoice');

  if (!currentInvoice) {
    alert('Nomor invoice tidak ditemukan.');
    window.location.href = '/';
    return;
  }

  document.getElementById('invDisplay').textContent = currentInvoice;

  await checkStatus();

  // Start status polling every 3 seconds
  pollInterval = setInterval(checkStatus, 3000);
}

async function checkStatus() {
  if (!currentInvoice) return;

  try {
    const res = await fetch(`/api/orders/${encodeURIComponent(currentInvoice)}/status`);
    if (!res.ok) return;

    const json = await res.json();
    if (!json.success || !json.data) return;

    const order = json.data;
    currentClaimCode = order.claim_code;
    document.getElementById('claimDisplay').textContent = order.claim_code;

    // Sync order state into browser's Pesanan Saya
    if (window.StoreKuOrders) {
      StoreKuOrders.saveOrder({
        invoice_number: order.invoice_number,
        claim_code: order.claim_code,
        product_name: order.product_name,
        quantity: order.quantity,
        total_amount: order.total_amount,
        payment_status: order.payment_status,
        expires_at: order.expires_at
      });
    }

    if (order.customer_email) {
      const emailNotice = document.getElementById('emailNotice');
      const emailDisplay = document.getElementById('buyerEmailDisplay');
      if (emailNotice && emailDisplay) {
        emailDisplay.textContent = order.customer_email;
        emailNotice.style.display = 'block';
      }
    }

    // Check if Paid and Fulfilled
    if (order.payment_status === 'PAID') {
      clearInterval(pollInterval);
      clearInterval(timerInterval);
      if (window.StoreKuOrders) StoreKuOrders.updateOrderStatus(order.invoice_number, 'PAID');
      showSuccessView(order);
      return;
    }

    if (order.payment_status === 'EXPIRED') {
      clearInterval(pollInterval);
      clearInterval(timerInterval);
      if (window.StoreKuOrders) StoreKuOrders.updateOrderStatus(order.invoice_number, 'EXPIRED');
      showExpiredView();
      return;
    }

    if (order.payment_status === 'CANCELLED') {
      clearInterval(pollInterval);
      clearInterval(timerInterval);
      if (window.StoreKuOrders) StoreKuOrders.updateOrderStatus(order.invoice_number, 'CANCELLED');
      showCancelledView();
      return;
    }

    // Render Pending View
    renderPendingView(order);

  } catch (err) {
    console.warn('Poll error:', err);
  }
}

function renderPendingView(order) {
  document.getElementById('totalAmountText').textContent = formatIDR(order.total_amount);
  
  if (order.qr_url) {
    const qrisImg = document.getElementById('qrisImage');
    if (qrisImg.src !== order.qr_url) {
      qrisImg.src = order.qr_url;
    }
  }

  // Setup countdown timer
  if (order.expires_at && !timerInterval) {
    startCountdown(new Date(order.expires_at).getTime());
  }
}

function startCountdown(targetTime) {
  const timerText = document.getElementById('timerText');

  function update() {
    const now = new Date().getTime();
    const distance = targetTime - now;

    if (distance <= 0) {
      clearInterval(timerInterval);
      if (window.StoreKuOrders) StoreKuOrders.updateOrderStatus(currentInvoice, 'EXPIRED');
      showExpiredView();
      return;
    }

    const minutes = Math.floor((distance % (1000 * 60 * 60)) / (1000 * 60));
    const seconds = Math.floor((distance % (1000 * 60)) / 1000);

    timerText.textContent = `${minutes < 10 ? '0' : ''}${minutes}:${seconds < 10 ? '0' : ''}${seconds}`;
  }

  update();
  timerInterval = setInterval(update, 1000);
}

function showSuccessView(order) {
  document.getElementById('pendingView').style.display = 'none';
  document.getElementById('expiredView').style.display = 'none';
  const cancelledView = document.getElementById('cancelledView');
  if (cancelledView) cancelledView.style.display = 'none';
  const successView = document.getElementById('successView');
  successView.style.display = 'block';

  document.getElementById('sProdName').textContent = order.product_name;
  document.getElementById('sProdWarranty').textContent = order.warranty_info ? `🛡️ ${order.warranty_info}` : '🛡️ Garansi Toko';

  if (order.customer_email) {
    const emailNotice = document.getElementById('emailNotice');
    if (emailNotice) {
      emailNotice.style.background = 'rgba(16, 185, 129, 0.12)';
      emailNotice.style.borderColor = 'rgba(16, 185, 129, 0.35)';
      emailNotice.style.color = '#34d399';
      emailNotice.innerHTML = `✉️ Kode klaim & rincian pesanan telah dikirim ke email: <strong style="color:#fff;">${escapeHtml(order.customer_email)}</strong>`;
      emailNotice.style.display = 'block';
    }
  }

  deliveredKeys = order.delivered_items || [];
  const itemsContainer = document.getElementById('itemsContainer');
  const copyAllBtn = document.getElementById('copyAllKeysBtn');

  if (order.is_quota_out || order.fulfillment_status === 'HOLD_PRICE_SPIKE' || order.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE' || order.fulfillment_status === 'FAILED') {
    if (copyAllBtn) copyAllBtn.style.display = 'none';
    const isFailed = order.fulfillment_status === 'FAILED' || order.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE';
    const title = isFailed ? 'Kendala Pemenuhan Stok Upstream' : 'Kuota Produk Habis Terjual';
    const defaultMsg = isFailed
      ? "Pembayaran Anda terverifikasi, namun terjadi kendala pemenuhan otomatis dari penyedia server (stok mendadak kosong/saldo bot). Dana Anda aman 100%! Silakan klik tombol di bawah untuk chat CS Telegram kami."
      : "Mohon maaf, kuota produk saat ini telah habis terjual. Dana pembayaran Anda aman 100%. Untuk proses pengembalian dana (refund), silakan hubungi Customer Service kami via Telegram.";

    itemsContainer.innerHTML = `
      <div style="background: rgba(245, 158, 11, 0.08); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: 12px; padding: 20px; text-align: center;">
        <div style="font-size: 1.8rem; margin-bottom: 8px;">⚠️</div>
        <div style="font-weight: 700; color: #fbbf24; font-size: 1.05rem; margin-bottom: 8px;">${title}</div>
        <p style="color: var(--text-muted); font-size: 0.88rem; line-height: 1.6; margin-bottom: 16px;">
          ${escapeHtml(order.quota_out_message || defaultMsg)}
        </p>
        <a href="https://t.me/Digihubsupport" target="_blank" rel="noopener" class="btn btn-primary" style="display: inline-flex; align-items: center; gap: 8px; font-size: 0.88rem; padding: 10px 20px;" data-tele-cs>
          <span>Hubungi CS via Telegram</span> ➔
        </a>
      </div>
    `;
    showToast(isFailed ? 'Kendala Stok Upstream' : 'Pembayaran Diterima (Kuota Habis)');
    return;
  }

  if (deliveredKeys.length === 0) {
    itemsContainer.innerHTML = `
      <div style="background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: 10px; padding: 12px; color: #fbbf24; font-size: 0.88rem;">
        Pembayaran terverifikasi! Pesanan Anda sedang diproses oleh antrean sistem dalam 1-2 menit. Silakan tunggu sebentar atau simpan Kode Klaim: <strong>${order.claim_code}</strong>.
      </div>
    `;
  } else {
    if (copyAllBtn) copyAllBtn.style.display = 'inline-block';
    itemsContainer.innerHTML = deliveredKeys.map((item, idx) => `
      <div class="license-item-box">
        <span>${escapeHtml(item)}</span>
        <button class="copy-mini-btn" onclick="copyText('${escapeHtml(item)}')">Salin</button>
      </div>
    `).join('');
  }

  showToast('Pembayaran Berhasil! 🎉');
}


function showExpiredView() {
  document.getElementById('pendingView').style.display = 'none';
  document.getElementById('successView').style.display = 'none';
  const cancelledView = document.getElementById('cancelledView');
  if (cancelledView) cancelledView.style.display = 'none';
  document.getElementById('expiredView').style.display = 'block';
}

function showCancelledView() {
  document.getElementById('pendingView').style.display = 'none';
  document.getElementById('successView').style.display = 'none';
  document.getElementById('expiredView').style.display = 'none';
  const cancelledView = document.getElementById('cancelledView');
  if (cancelledView) cancelledView.style.display = 'block';
}

function openCancelModal() {
  document.getElementById('cancelConfirmModal')?.classList.add('open');
}

function closeCancelModal() {
  document.getElementById('cancelConfirmModal')?.classList.remove('open');
}

async function confirmCancelOrder() {
  const btn = document.getElementById('btnConfirmCancelAction');
  if (btn) {
    btn.disabled = true;
    btn.textContent = 'Membatalkan...';
  }

  try {
    const res = await fetch(`/api/orders/${encodeURIComponent(currentInvoice)}/cancel`, {
      method: 'POST'
    });
    const json = await res.json();

    if (!json.success) {
      showToast(json.error || 'Gagal membatalkan pesanan.');
      if (btn) {
        btn.disabled = false;
        btn.textContent = 'Ya, Batalkan';
      }
      return;
    }

    closeCancelModal();
    clearInterval(pollInterval);
    clearInterval(timerInterval);

    if (window.StoreKuOrders) {
      StoreKuOrders.updateOrderStatus(currentInvoice, 'CANCELLED');
    }

    showCancelledView();
    showToast('Pesanan berhasil dibatalkan. ✕');
  } catch (err) {
    showToast('Koneksi terputus: ' + err.message);
    if (btn) {
      btn.disabled = false;
      btn.textContent = 'Ya, Batalkan';
    }
  }
}

// "Saya Sudah Bayar" instant trigger
async function triggerManualCheck() {
  const btn = document.getElementById('btnManualCheck');
  if (!btn || !currentInvoice) return;

  btn.disabled = true;
  btn.innerText = 'Memeriksa mutasi ShopeePay...';

  try {
    const res = await fetch(`/api/orders/${encodeURIComponent(currentInvoice)}/check`, {
      method: 'POST'
    });
    const json = await res.json();
    if (json.success && json.data) {
      if (json.data.payment_status === 'PAID') {
        if (window.StoreKuOrders) StoreKuOrders.updateOrderStatus(currentInvoice, 'PAID');
        showSuccessView(json.data);
        return;
      }
    }
    showToast('Mutasi belum terdeteksi. Silakan tunggu 10-30 detik.');
  } catch (e) {
    showToast('Gagal memeriksa mutasi: ' + e.message);
  } finally {
    setTimeout(() => {
      btn.disabled = false;
      btn.innerText = '⚡ Saya Sudah Bayar (Cek Mutasi Sekarang)';
    }, 4000);
  }
}

function copyAllKeys() {
  if (deliveredKeys.length === 0) return;
  copyText(deliveredKeys.join('\n'));
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str).replace(/[&<>'"]/g, tag => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
  }[tag] || tag));
}

document.addEventListener('DOMContentLoaded', () => {
  loadPaymentDetails();

  document.getElementById('btnManualCheck')?.addEventListener('click', triggerManualCheck);
  document.getElementById('btnCheckExpiredLate')?.addEventListener('click', triggerManualCheck);
  document.getElementById('copyAllKeysBtn')?.addEventListener('click', copyAllKeys);

  // Cancel order modal handlers
  document.getElementById('btnCancelOrder')?.addEventListener('click', openCancelModal);
  document.getElementById('btnDismissCancelModal')?.addEventListener('click', closeCancelModal);
  document.getElementById('btnConfirmCancelAction')?.addEventListener('click', confirmCancelOrder);
  document.getElementById('cancelConfirmModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'cancelConfirmModal') closeCancelModal();
  });
});
