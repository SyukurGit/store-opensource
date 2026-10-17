// Digimest Payment Page Engine

let pollInterval = null;
let timerInterval = null;
let currentInvoice = '';
let currentClaimCode = '';
let deliveredKeys = [];

function formatIDR(num) {
  if (window.DigimestCurrency && typeof window.DigimestCurrency.format === 'function') {
    return window.DigimestCurrency.format(num);
  }
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
  const isEn = document.documentElement.lang === 'en';
  navigator.clipboard.writeText(text).then(() => {
    showToast(isEn ? 'Copied to clipboard! 📋' : 'Berhasil disalin ke clipboard! 📋');
  }).catch(() => {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    showToast(isEn ? 'Copied to clipboard!' : 'Berhasil disalin!');
  });
}

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

function getOrderHandler() {
  return window.DigimestOrders || window.DigihubOrders || window.StoreKuOrders;
}

async function loadPaymentDetails() {
  const params = new URLSearchParams(window.location.search);
  currentInvoice = params.get('inv') || params.get('invoice');

  const isEn = (document.documentElement && document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');
  if (!currentInvoice) {
    alert(isEn ? 'Invoice number not found.' : 'Nomor invoice tidak ditemukan.');
    window.location.href = isEn ? '/en/' : '/';
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
    const statusUrl = window.digimestApi ? window.digimestApi(`/api/orders/${encodeURIComponent(currentInvoice)}/status`) : `/api/orders/${encodeURIComponent(currentInvoice)}/status`;
    const res = await fetch(statusUrl);
    if (!res.ok) return;

    const json = await res.json();
    if (!json.success || !json.data) return;

    const order = json.data;
    currentClaimCode = order.claim_code || '';

    // Update status badge di header
    const statusBadge = document.getElementById('statusBadgeDisplay');
    const isEn = document.documentElement.lang === 'en';
    if (statusBadge) {
      if (order.payment_status === 'PAID') {
        statusBadge.innerHTML = isEn ? '● Paid' : '● Lunas';
        statusBadge.style.background = 'rgba(16, 185, 129, 0.15)';
        statusBadge.style.color = '#34d399';
        statusBadge.style.borderColor = 'rgba(16, 185, 129, 0.35)';
      } else if (order.payment_status === 'EXPIRED') {
        statusBadge.innerHTML = isEn ? '⌛ Expired' : '⌛ Kedaluwarsa';
        statusBadge.style.background = 'rgba(239, 68, 68, 0.15)';
        statusBadge.style.color = '#f87171';
        statusBadge.style.borderColor = 'rgba(239, 68, 68, 0.35)';
      } else if (order.payment_status === 'CANCELLED') {
        statusBadge.innerHTML = isEn ? '✕ Cancelled' : '✕ Dibatalkan';
        statusBadge.style.background = 'rgba(148, 163, 184, 0.15)';
        statusBadge.style.color = '#94a3b8';
        statusBadge.style.borderColor = 'rgba(148, 163, 184, 0.35)';
      } else {
        statusBadge.innerHTML = isEn ? '⏳ Awaiting Payment' : '⏳ Menunggu Pembayaran';
        statusBadge.style.background = 'rgba(245, 158, 11, 0.15)';
        statusBadge.style.color = '#fbbf24';
        statusBadge.style.borderColor = 'rgba(245, 158, 11, 0.35)';
      }
    }

    // Sync order state into browser's Pesanan Saya
    const handler = getOrderHandler();
    if (handler) {
      handler.saveOrder({
        invoice_number: order.invoice_number,
        claim_code: order.claim_code || '',
        product_name: order.product_name,
        quantity: order.quantity,
        total_amount: order.total_amount,
        payment_status: order.payment_status,
        expires_at: order.expires_at,
        created_at: order.created_at
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
      if (handler) handler.updateOrderStatus(order.invoice_number, 'PAID');
      showSuccessView(order);
      return;
    }

    if (order.payment_status === 'EXPIRED') {
      clearInterval(pollInterval);
      clearInterval(timerInterval);
      if (handler) handler.updateOrderStatus(order.invoice_number, 'EXPIRED');
      showExpiredView();
      return;
    }

    if (order.payment_status === 'CANCELLED') {
      clearInterval(pollInterval);
      clearInterval(timerInterval);
      if (handler) handler.updateOrderStatus(order.invoice_number, 'CANCELLED');
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
  const isBinance = (order.payment_method === 'BINANCE_PAY' || order.payment_gateway === 'BINANCE');
  const isBEP20 = (order.payment_gateway === 'USDT_BEP20' || order.payment_channel === 'BEP20' || order.payment_gateway === 'PAYMENTG_BEP20' || (order.payment_method === 'CRYPTO' && !isBinance));
  const qrisBox = document.getElementById('qrisPendingContainer');
  const binanceBox = document.getElementById('binancePendingContainer');
  const bep20Box = document.getElementById('bep20PendingContainer');

  if (isBEP20) {
    if (qrisBox) qrisBox.style.display = 'none';
    if (binanceBox) binanceBox.style.display = 'none';
    if (bep20Box) bep20Box.style.display = 'block';

    if (order.qr_url) {
      const qrImg = document.getElementById('bep20QrCode');
      if (qrImg && qrImg.src !== order.qr_url) {
        qrImg.src = order.qr_url;
      }
    }

    const bepAmount = document.getElementById('bep20AmountDisplay');
    if (bepAmount && order.crypto_amount_str) {
      bepAmount.textContent = order.crypto_amount_str;
    }
    const bepCexAmount = document.getElementById('bep20CexAmount');
    if (bepCexAmount && order.crypto_amount_str) {
      bepCexAmount.textContent = order.crypto_amount_str;
    }
    const bepWallet = document.getElementById('bep20WalletDisplay');
    if (bepWallet && order.wallet_address) {
      bepWallet.textContent = order.wallet_address;
    }
  } else if (isBinance) {
    if (qrisBox) qrisBox.style.display = 'none';
    if (bep20Box) bep20Box.style.display = 'none';
    if (binanceBox) binanceBox.style.display = 'block';

    if (order.qr_url) {
      const qrImg = document.getElementById('binanceQrCode');
      if (qrImg && qrImg.src !== order.qr_url) {
        qrImg.src = order.qr_url;
      }
    }

    const cryptoAmt = document.getElementById('cryptoAmountDisplay');
    if (cryptoAmt && order.crypto_amount_str) {
      cryptoAmt.textContent = order.crypto_amount_str;
    }
    const cryptoCurr = document.getElementById('cryptoCurrencyDisplay');
    if (cryptoCurr && order.crypto_currency) {
      cryptoCurr.textContent = order.crypto_currency;
    }
    const payId = document.getElementById('binancePayIdDisplay');
    if (payId && order.binance_pay_id) {
      payId.textContent = order.binance_pay_id;
    }
    const accName = document.getElementById('binanceAccountNameDisplay');
    if (accName && order.binance_account_name) {
      accName.textContent = order.binance_account_name;
    }
  } else {
    if (binanceBox) binanceBox.style.display = 'none';
    if (bep20Box) bep20Box.style.display = 'none';
    if (qrisBox) qrisBox.style.display = 'block';

    const amtText = document.getElementById('totalAmountText');
    if (amtText) amtText.textContent = formatIDR(order.total_amount);

    if (order.qr_url) {
      const qrisImg = document.getElementById('qrisImage');
      if (qrisImg && qrisImg.src !== order.qr_url) {
        qrisImg.src = order.qr_url;
      }
    }
  }

  // Setup countdown timer
  if (!timerInterval) {
    let targetTime = parseTimestamp(order.expires_at);
    if (!targetTime && order.created_at) {
      const createdTime = parseTimestamp(order.created_at);
      if (createdTime) targetTime = createdTime + 15 * 60 * 1000;
    }
    if (!targetTime) {
      targetTime = Date.now() + 15 * 60 * 1000;
    }
    startCountdown(targetTime);
  }
}

function startCountdown(targetTime) {
  const timerText = document.getElementById('timerText');

  function update() {
    const now = new Date().getTime();
    const distance = targetTime - now;

    if (distance <= 0) {
      clearInterval(timerInterval);
      clearInterval(pollInterval);
      const handler = getOrderHandler();
      if (handler) handler.updateOrderStatus(currentInvoice, 'EXPIRED');
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

  const isEn = document.documentElement.lang === 'en';
  document.getElementById('sProdName').textContent = order.product_name;
  document.getElementById('sProdWarranty').textContent = order.warranty_info ? `🛡️ ${order.warranty_info}` : (isEn ? '🛡️ Store Warranty' : '🛡️ Garansi Toko');

  currentClaimCode = order.claim_code || '';

  // Setup KARTU KODE AMBIL PESANAN & DIRECT LINK
  const paidCodeEl = document.getElementById('paidClaimCodeDisplay');
  if (paidCodeEl) {
    paidCodeEl.textContent = currentClaimCode || '-';
  }

  const directLinkEl = document.getElementById('paidDirectClaimLink');
  const directUrlTextEl = document.getElementById('paidDirectClaimUrlText');
  if (currentClaimCode) {
    const claimPath = isEn ? '/en/claim' : '/claim';
    const directUrl = `${window.location.origin}${claimPath}?code=${encodeURIComponent(currentClaimCode)}`;
    if (directLinkEl) {
      directLinkEl.href = directUrl;
    }
    if (directUrlTextEl) {
      directUrlTextEl.textContent = directUrl;
    }
  }

  const copyClaimBtn = document.getElementById('btnCopyClaimCode');
  if (copyClaimBtn && currentClaimCode) {
    copyClaimBtn.onclick = () => {
      copyText(currentClaimCode);
    };
  }

  if (order.customer_email) {
    const emailNotice = document.getElementById('emailNotice');
    if (emailNotice) {
      emailNotice.style.background = 'rgba(16, 185, 129, 0.12)';
      emailNotice.style.borderColor = 'rgba(16, 185, 129, 0.35)';
      emailNotice.style.color = '#34d399';
      emailNotice.innerHTML = isEn 
        ? `✉️ Order pickup code & details have been sent to email: <strong style="color:#fff;">${escapeHtml(order.customer_email)}</strong>`
        : `✉️ Kode ambil pesanan & rincian telah dikirim ke email: <strong style="color:#fff;">${escapeHtml(order.customer_email)}</strong>`;
      emailNotice.style.display = 'block';
    }
  }

  deliveredKeys = order.delivered_items || [];
  const itemsContainer = document.getElementById('itemsContainer');
  const copyAllBtn = document.getElementById('copyAllKeysBtn');

  if (order.is_quota_out || order.fulfillment_status === 'HOLD_PRICE_SPIKE' || order.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE' || order.fulfillment_status === 'FAILED') {
    if (copyAllBtn) copyAllBtn.style.display = 'none';
    const isFailed = order.fulfillment_status === 'FAILED' || order.fulfillment_status === 'HOLD_INSUFFICIENT_BOT_BALANCE';
    const title = isEn 
      ? (isFailed ? 'Upstream Stock Delivery Issue' : 'Product Quota Sold Out')
      : (isFailed ? 'Kendala Pemenuhan Stok Upstream' : 'Kuota Produk Habis Terjual');
    const defaultMsg = isEn
      ? (isFailed
        ? "Your payment is verified, but an automatic fulfillment issue occurred with the server provider (momentarily empty stock/bot balance). Your funds are 100% safe! Please contact our Telegram CS below."
        : "Sorry, this product quota is currently sold out. Your funds are 100% safe. For refund or assistance, please contact our Telegram Customer Service.")
      : (isFailed
        ? "Pembayaran Anda terverifikasi, namun terjadi kendala pemenuhan otomatis dari penyedia server (stok mendadak kosong/saldo bot). Dana Anda aman 100%! Silakan klik tombol di bawah untuk chat CS Telegram kami."
        : "Mohon maaf, kuota produk saat ini telah habis terjual. Dana pembayaran Anda aman 100%. Untuk proses pengembalian dana (refund), silakan hubungi Customer Service kami via Telegram.");

    itemsContainer.innerHTML = `
      <div style="background: rgba(245, 158, 11, 0.08); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: 12px; padding: 20px; text-align: center;">
        <div style="font-size: 1.8rem; margin-bottom: 8px;">⚠️</div>
        <div style="font-weight: 700; color: #fbbf24; font-size: 1.05rem; margin-bottom: 8px;">${title}</div>
        <p style="color: var(--text-muted); font-size: 0.88rem; line-height: 1.6; margin-bottom: 16px;">
          ${escapeHtml(order.quota_out_message || defaultMsg)}
        </p>
        <a href="https://t.me/Digimestsupport" target="_blank" rel="noopener" class="btn btn-primary" style="display: inline-flex; align-items: center; gap: 8px; font-size: 0.88rem; padding: 10px 20px;" data-tele-cs>
          <span>${isEn ? 'Contact CS via Telegram' : 'Hubungi CS via Telegram'}</span> ➔
        </a>
      </div>
    `;
    showToast(isEn ? (isFailed ? 'Upstream Stock Issue' : 'Payment Received (Sold Out)') : (isFailed ? 'Kendala Stok Upstream' : 'Pembayaran Diterima (Kuota Habis)'));
    return;
  }

  if (deliveredKeys.length === 0) {
    itemsContainer.innerHTML = `
      <div style="background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.3); border-radius: 10px; padding: 12px; color: #fbbf24; font-size: 0.88rem;">
        ${isEn
          ? 'Payment verified! Your order is being processed by the system queue in 1-2 minutes. Please wait a moment or save the Pickup Code above.'
          : 'Pembayaran terverifikasi! Pesanan Anda sedang diproses oleh antrean sistem dalam 1-2 menit. Silakan tunggu sebentar atau simpan Kode Ambil Pesanan di atas.'}
      </div>
    `;
  } else {
    if (copyAllBtn) copyAllBtn.style.display = 'inline-block';
    itemsContainer.innerHTML = deliveredKeys.map((item, idx) => `
      <div class="license-item-box">
        <span>${escapeHtml(item)}</span>
        <button class="copy-mini-btn" onclick="copyText('${escapeHtml(item)}')">${isEn ? 'Copy' : 'Salin'}</button>
      </div>
    `).join('');
  }

  const existingTx = document.getElementById('successTxHashRow');
  if (existingTx) existingTx.remove();
  if (order.tx_hash) {
    const txRow = document.createElement('div');
    txRow.id = 'successTxHashRow';
    txRow.style.cssText = 'background: rgba(240, 185, 11, 0.08); border: 1px solid rgba(240, 185, 11, 0.25); border-radius: 10px; padding: 10px 14px; margin-top: 1rem; font-size: 0.8rem; display: flex; justify-content: space-between; align-items: center; gap: 8px; flex-wrap: wrap; text-align: left;';
    txRow.innerHTML = `
      <div style="display: flex; align-items: center; gap: 6px;">
        <span>🟡</span>
        <span style="color: #94a3b8;">${isEn ? 'Blockchain Tx Hash (BSC):' : 'Hash Transaksi Blockchain (BSC):'}</span>
      </div>
      <a href="https://bscscan.com/tx/${encodeURIComponent(order.tx_hash)}" target="_blank" rel="noopener noreferrer" style="color: #f0b90b; font-family: var(--font-mono); font-weight: 700; text-decoration: underline; font-size: 0.78rem;" title="${escapeHtml(order.tx_hash)}">
        ${escapeHtml(order.tx_hash.slice(0, 10))}...${escapeHtml(order.tx_hash.slice(-8))} ↗
      </a>
    `;
    itemsContainer.parentNode.appendChild(txRow);
  }

  showToast(isEn ? 'Payment Successful! 🎉' : 'Pembayaran Berhasil! 🎉');
}


function showExpiredView() {
  clearInterval(pollInterval);
  clearInterval(timerInterval);
  document.getElementById('pendingView').style.display = 'none';
  document.getElementById('successView').style.display = 'none';
  const cancelledView = document.getElementById('cancelledView');
  if (cancelledView) cancelledView.style.display = 'none';
  document.getElementById('expiredView').style.display = 'block';
}

function showCancelledView() {
  clearInterval(pollInterval);
  clearInterval(timerInterval);
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
  const isEn = document.documentElement.lang === 'en';
  const btn = document.getElementById('btnConfirmCancelAction');
  if (btn) {
    btn.disabled = true;
    btn.textContent = isEn ? 'Cancelling...' : 'Membatalkan...';
  }

  try {
    const cancelUrl = window.digimestApi ? window.digimestApi(`/api/orders/${encodeURIComponent(currentInvoice)}/cancel`) : `/api/orders/${encodeURIComponent(currentInvoice)}/cancel`;
    const res = await fetch(cancelUrl, {
      method: 'POST'
    });
    const json = await res.json();

    if (!json.success) {
      showToast(json.error || (isEn ? 'Failed to cancel order.' : 'Gagal membatalkan pesanan.'));
      if (btn) {
        btn.disabled = false;
        btn.textContent = isEn ? 'Yes, Cancel' : 'Ya, Batalkan';
      }
      return;
    }

    closeCancelModal();
    clearInterval(pollInterval);
    clearInterval(timerInterval);

    const handler = getOrderHandler();
    if (handler) {
      handler.updateOrderStatus(currentInvoice, 'CANCELLED');
    }

    showCancelledView();
    showToast(isEn ? 'Order cancelled successfully. ✕' : 'Pesanan berhasil dibatalkan. ✕');
  } catch (err) {
    showToast((isEn ? 'Connection error: ' : 'Koneksi terputus: ') + err.message);
    if (btn) {
      btn.disabled = false;
      btn.textContent = isEn ? 'Yes, Cancel' : 'Ya, Batalkan';
    }
  }
}

// "Saya Sudah Bayar" instant trigger (dynamically handles either pending or expired late button)
async function triggerManualCheck(e) {
  const isEn = document.documentElement.lang === 'en';
  const targetBtn = (e && e.currentTarget) ? e.currentTarget : document.getElementById('btnManualCheck');
  if (!currentInvoice) return;

  const spinner = document.getElementById('manualCheckSpinner');
  const icon = document.getElementById('manualCheckIcon');
  const textEl = document.getElementById('manualCheckText');

  if (spinner) spinner.style.display = 'inline-block';
  if (icon) icon.style.display = 'none';
  if (textEl) {
    textEl.textContent = isEn ? 'Checking payment mutation...' : 'Memeriksa mutasi pembayaran...';
  } else if (targetBtn) {
    targetBtn.innerText = isEn ? 'Checking payment mutation...' : 'Memeriksa mutasi pembayaran...';
  }

  if (targetBtn) {
    targetBtn.disabled = true;
  }

  try {
    const checkUrl = window.digimestApi ? window.digimestApi(`/api/orders/${encodeURIComponent(currentInvoice)}/check`) : `/api/orders/${encodeURIComponent(currentInvoice)}/check`;
    const res = await fetch(checkUrl, {
      method: 'POST'
    });
    const json = await res.json();
    if (json.success && json.data) {
      if (json.data.payment_status === 'PAID') {
        const handler = getOrderHandler();
        if (handler) handler.updateOrderStatus(currentInvoice, 'PAID');
        showSuccessView(json.data);
        return;
      }
    }
    // Perform full status poll update
    await checkStatus();
    showToast(json.message || (isEn ? 'Payment not detected yet. Please wait 10-30 seconds.' : 'Mutasi belum terdeteksi. Silakan tunggu 10-30 detik.'));
  } catch (err) {
    showToast((isEn ? 'Failed to check payment: ' : 'Gagal memeriksa mutasi: ') + err.message);
  } finally {
    // Cooldown 4 seconds to match server throttle
    setTimeout(() => {
      if (spinner) spinner.style.display = 'none';
      if (icon) icon.style.display = 'inline-block';
      if (textEl) {
        textEl.textContent = isEn ? 'I Have Paid (Check Status Now)' : 'Saya Sudah Bayar (Cek Mutasi Sekarang)';
      } else if (targetBtn) {
        targetBtn.innerText = isEn ? '⚡ Check Status' : '⚡ Periksa Status';
      }
      if (targetBtn) {
        targetBtn.disabled = false;
      }
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

  // Copy buttons for Binance Pay
  document.getElementById('btnCopyCryptoAmount')?.addEventListener('click', () => {
    const el = document.getElementById('cryptoAmountDisplay');
    if (el) copyText(el.textContent.trim());
  });
  document.getElementById('btnCopyPayId')?.addEventListener('click', () => {
    const el = document.getElementById('binancePayIdDisplay');
    if (el) copyText(el.textContent.trim());
  });

  // Copy buttons for USDT BEP-20
  document.getElementById('btnCopyBep20Amount')?.addEventListener('click', () => {
    const el = document.getElementById('bep20AmountDisplay');
    if (el) copyText(el.textContent.trim());
  });
  document.getElementById('btnCopyBep20Wallet')?.addEventListener('click', () => {
    const el = document.getElementById('bep20WalletDisplay');
    if (el) copyText(el.textContent.trim());
  });

  // Cancel order modal handlers
  document.getElementById('btnCancelOrder')?.addEventListener('click', openCancelModal);
  document.getElementById('btnDismissCancelModal')?.addEventListener('click', closeCancelModal);
  document.getElementById('btnConfirmCancelAction')?.addEventListener('click', confirmCancelOrder);
  document.getElementById('cancelConfirmModal')?.addEventListener('click', (e) => {
    if (e.target.id === 'cancelConfirmModal') closeCancelModal();
  });

  // Pause polling when tab is hidden, resume when tab is active
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      clearInterval(pollInterval);
    } else {
      const isPending = document.getElementById('pendingView')?.style.display !== 'none';
      if (currentInvoice && isPending) {
        checkStatus();
        clearInterval(pollInterval);
        pollInterval = setInterval(checkStatus, 3000);
      }
    }
  });
});
