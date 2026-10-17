// Digimest Product Detail JavaScript
let currentProduct = null;
let currentQty = 1;
let appliedVoucher = null;
let selectedPaymentGateway = 'PAYMENTG';
let selectedPaymentMethod = 'QRIS';
let selectedPaymentChannel = 'SHOPEEPAY';
let appConfig = { usd_rate: 18000 };

document.addEventListener('DOMContentLoaded', () => {
  initProductPage();
  setupEventListeners();
});

function formatCurrency(amountIDR, amountUSD = 0) {
  if (window.DigimestCurrency && typeof window.DigimestCurrency.format === 'function') {
    return window.DigimestCurrency.format(amountIDR, amountUSD);
  }
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0
  }).format(amountIDR || 0);
}

function formatIDR(num) {
  return formatCurrency(num, 0);
}

function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 2500);
}

function updateUsdRateDisplays() {
  const rateText = (appConfig.usd_rate || 18000).toLocaleString('id-ID');
  const el1 = document.getElementById('mUsdRateText');
  if (el1) el1.textContent = rateText;
  const el2 = document.getElementById('mSummaryUsdRate');
  if (el2) el2.textContent = rateText;
}

async function initProductPage() {
  // Fetch dynamic usd_rate from app config
  fetch(window.digimestApi ? window.digimestApi('/api/config') : '/api/config')
    .then(r => r.json())
    .then(json => {
      const rate = (json && json.data && json.data.usd_rate) || (json && json.usd_rate);
      if (rate) {
        appConfig.usd_rate = parseFloat(rate) || 18000;
        updateUsdRateDisplays();
        updateCalculations();
      }
    })
    .catch(() => {});

  const params = new URLSearchParams(window.location.search);
  const rawSlug = params.get('slug');

  if (!rawSlug) {
    showError('Parameter Produk Kosong', 'Silakan pilih produk dari katalog toko terlebih dahulu.');
    return;
  }

  // Clean slug variations
  let cleanSlug = rawSlug;
  try {
    cleanSlug = decodeURIComponent(rawSlug);
  } catch (e) {}

  try {
    // 1. Try direct detail fetch with literal '+' (Chi router matches literal '+')
    const pathSlug = encodeURIComponent(cleanSlug).replace(/%2B/gi, '+');
    const detailUrl = window.digimestApi ? window.digimestApi(`/api/products/${pathSlug}`) : `/api/products/${pathSlug}`;
    let res = await fetch(detailUrl);

    if (!res.ok && pathSlug !== cleanSlug) {
      const altUrl = window.digimestApi ? window.digimestApi(`/api/products/${encodeURIComponent(cleanSlug)}`) : `/api/products/${encodeURIComponent(cleanSlug)}`;
      const altRes = await fetch(altUrl);
      if (altRes.ok) res = altRes;
    }

    if (res.ok) {
      const json = await res.json();
      if (json.success && json.data) {
        currentProduct = json.data;
        renderProductDetail(currentProduct, json.review_stats, json.reviews);
        return;
      }
    }

    // 2. SELF-HEALING FALLBACK: If direct slug fetch failed (404/encoding issue), fallback to /api/products catalog!
    console.warn('Direct detail lookup missed, activating catalog fallback for:', cleanSlug);
    const catalogUrl = window.digimestApi ? window.digimestApi('/api/products') : '/api/products';
    const catRes = await fetch(catalogUrl);
    if (catRes.ok) {
      const catJson = await catRes.json();
      const list = catJson.data || [];

      // Flexible matching: exact, unescaped, normalized (+ vs _ vs space)
      const normSlug = cleanSlug.toLowerCase().replace(/[^a-z0-9]/g, '');
      const found = list.find(p => {
        if (!p || !p.slug) return false;
        if (p.slug === cleanSlug || p.slug === rawSlug) return true;
        const normP = p.slug.toLowerCase().replace(/[^a-z0-9]/g, '');
        return normP === normSlug;
      });

      if (found) {
        currentProduct = found;
        let rStats = null, rList = [];
        try {
          const revUrl = window.digimestApi ? window.digimestApi(`/api/products/${encodeURIComponent(found.slug)}/reviews`) : `/api/products/${encodeURIComponent(found.slug)}/reviews`;
          const revRes = await fetch(revUrl);
          if (revRes.ok) {
            const revJson = await revRes.json();
            rStats = revJson.stats;
            rList = revJson.data;
          }
        } catch (e) {}
        renderProductDetail(currentProduct, rStats, rList);
        return;
      }
    }

    // Only if both direct fetch and catalog fallback completely fail
    const errData = await res.json().catch(() => ({}));
    showError('Produk Tidak Ditemukan', errData.error || `Produk '${cleanSlug}' tidak ditemukan atau sedang tidak aktif.`);

  } catch (err) {
    console.error('Fetch product error:', err);
    showError('Koneksi Gagal', 'Gagal menghubungi server Digimest. Silakan coba beberapa saat lagi.');
  }
}

function showError(title, msg) {
  document.getElementById('productLoading').style.display = 'none';
  document.getElementById('productDetailContent').style.display = 'none';
  const errBox = document.getElementById('productError');
  errBox.style.display = 'block';
  document.getElementById('errorTitle').textContent = title;
  document.getElementById('errorMessage').textContent = msg;
}

function formatDate(dateStr) {
  if (!dateStr) return '-';
  const d = new Date(dateStr);
  return d.toLocaleDateString('id-ID', { year: 'numeric', month: 'short', day: 'numeric' });
}

function getStarString(rating) {
  const rounded = Math.round(Number(rating || 5));
  let stars = '';
  for (let i = 1; i <= 5; i++) {
    stars += i <= rounded ? '★' : '☆';
  }
  return stars;
}

function renderReviewsSection(stats, reviews) {
  const avg = stats?.average_rating || 5.0;
  const count = stats?.total_reviews || (reviews ? reviews.length : 0);

  const avgEl = document.getElementById('dRatingAvgText');
  if (avgEl) avgEl.textContent = avg.toFixed(1);

  const isEn = (document.documentElement && document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');
  const countEl = document.getElementById('dRatingCountText');
  if (countEl) countEl.textContent = isEn ? `(${count} reviews)` : `(${count} ulasan)`;

  const scoreBig = document.getElementById('revOverallScore');
  if (scoreBig) scoreBig.textContent = avg.toFixed(1);

  const overallStars = document.getElementById('revOverallStars');
  if (overallStars) overallStars.textContent = getStarString(avg);

  const overallCount = document.getElementById('revOverallCount');
  if (overallCount) overallCount.textContent = isEn ? `Based on ${count} reviews` : `Berdasarkan ${count} ulasan`;

  // Star breakdown bars
  const barsWrap = document.getElementById('revBreakdownBars');
  if (barsWrap) {
    const dist = stats?.distribution || { 5: 0, 4: 0, 3: 0, 2: 0, 1: 0 };
    let html = '';
    for (let star = 5; star >= 1; star--) {
      const c = dist[star] || 0;
      const pct = count > 0 ? Math.round((c / count) * 100) : 0;
      html += `
        <div class="bar-row">
          <span class="bar-star-label">${star} ★</span>
          <div class="bar-track">
            <div class="bar-fill" style="width: ${pct}%;"></div>
          </div>
          <span class="bar-count">${c}</span>
        </div>
      `;
    }
    barsWrap.innerHTML = html;
  }

  // Reviews List
  const listWrap = document.getElementById('reviewsListWrap');
  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  if (listWrap) {
    if (!reviews || reviews.length === 0) {
      listWrap.innerHTML = `
        <div style="text-align: center; padding: 2rem 1rem; color: var(--text-muted); font-size: 0.88rem; background: rgba(255,255,255,0.015); border-radius: 10px; border: 1px dashed rgba(255,255,255,0.08);">
          ${isEn ? 'No customer reviews yet for this product.<br><span style="font-size: 0.8rem; opacity: 0.8;">Be the first buyer to review this product after completion!</span>' : 'Belum ada ulasan untuk produk ini.<br><span style="font-size: 0.8rem; opacity: 0.8;">Jadilah pembeli pertama yang memberikan penilaian setelah pesanan Anda lunas!</span>'}
        </div>
      `;
    } else {
      const renderCard = (r) => {
        const initial = (r.author_name || 'U').charAt(0).toUpperCase();
        const dateStr = formatDate(r.created_at);
        const starStr = getStarString(r.rating);
        const verifiedText = isEn ? '✓ Verified Buyer' : '✓ Terverifikasi';
        const defaultAuthor = isEn ? 'Verified Customer' : 'Pelanggan Terverifikasi';
        return `
          <div class="review-item-card">
            <div class="review-item-top">
              <div class="review-author-wrap">
                <div class="review-avatar">${escapeHtml(initial)}</div>
                <div class="review-author-name">${escapeHtml(r.author_name || defaultAuthor)}</div>
                <span class="verified-buyer-badge">${verifiedText}</span>
              </div>
              <div class="review-stars-val">${starStr}</div>
            </div>
            ${r.review_text ? `<div class="review-comment">${escapeHtml(r.review_text)}</div>` : ''}
            <div class="review-date-val">${escapeHtml(dateStr)}</div>
          </div>
        `;
      };

      if (reviews.length <= 5) {
        listWrap.innerHTML = reviews.map(renderCard).join('');
      } else {
        const firstBatch = reviews.slice(0, 5).map(renderCard).join('');
        const remainingBatch = reviews.slice(5).map(renderCard).join('');
        const btnMoreText = isEn ? `View All Reviews (${reviews.length}) ▾` : `Lihat Seluruh Ulasan (${reviews.length}) ▾`;
        listWrap.innerHTML = `
          ${firstBatch}
          <div id="hiddenReviewsContainer" style="display: none;">
            ${remainingBatch}
          </div>
          <div id="moreReviewsWrap" style="text-align: center; margin-top: 1.25rem;">
            <button type="button" id="btnShowMoreReviews" class="btn-nav" style="display: inline-flex; align-items: center; gap: 6px; padding: 0.55rem 1.25rem; font-size: 0.85rem; color: #fbbf24; border-color: rgba(251, 191, 36, 0.35);">
              ${btnMoreText}
            </button>
          </div>
        `;

        document.getElementById('btnShowMoreReviews')?.addEventListener('click', () => {
          const hiddenCont = document.getElementById('hiddenReviewsContainer');
          const btnMore = document.getElementById('btnShowMoreReviews');
          if (hiddenCont) hiddenCont.style.display = 'block';
          if (btnMore) btnMore.parentElement.style.display = 'none';
        });
      }
    }
  }
}

function renderProductDetail(p, stats, reviews) {
  document.getElementById('productLoading').style.display = 'none';
  const detailGrid = document.getElementById('productDetailContent');
  if (detailGrid) {
    detailGrid.style.display = window.innerWidth <= 860 ? 'flex' : 'grid';
  }

  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  // Page title & Breadcrumb
  document.title = `${p.name} - Digimest`;
  document.getElementById('bcProductName').textContent = p.name;

  // Header
  document.getElementById('dProdLogo').src = p.logo || 'https://via.placeholder.com/76?text=DC';
  const titleEl = document.getElementById('dProdTitle');
  if (titleEl) titleEl.textContent = p.name;
  const slugEl = document.getElementById('dProdSlug');
  if (slugEl) slugEl.textContent = p.slug;

  // Sold count & Reviews
  const soldEl = document.getElementById('dSoldText');
  if (soldEl) soldEl.textContent = `${p.salesCount || 10} ${isEn ? 'sold' : 'terjual'}`;
  renderReviewsSection(stats, reviews);

  // Badges
  document.getElementById('dProdCategory').textContent = p.type || 'Digital License';
  document.getElementById('dProdRegion').textContent = p.region ? `🌐 ${p.region}` : '🌐 Global';
  document.getElementById('dProdWarranty').textContent = p.warranty ? `🛡️ ${p.warranty}` : (isEn ? '🛡️ Store Warranty' : '🛡️ Garansi Toko');

  const isOut = p.stock <= 0 || !!p.isForceOutOfStock;
  const stockText = isOut
    ? (isEn ? 'Out of Stock' : 'Habis')
    : (p.stockCapped ? (isEn ? '>1000 In Stock' : '>1000 Stok') : (isEn ? `${p.stock} In Stock` : `${p.stock} Stok`));
  const stockBadge = document.getElementById('dProdStock');
  stockBadge.textContent = `● ${stockText}`;
  if (isOut) stockBadge.classList.add('empty');

  const dStockNoteEl = document.getElementById('dStockNote');
  if (dStockNoteEl) {
    dStockNoteEl.textContent = isOut ? (isEn ? '● Out of Stock' : '● Stok Habis') : (isEn ? `● Available: ${stockText}` : `● Tersedia: ${stockText}`);
    if (isOut) {
      dStockNoteEl.classList.add('empty');
    } else {
      dStockNoteEl.classList.remove('empty');
    }
  }
  const modalStock = document.getElementById('mModalStockNote');
  if (modalStock) {
    modalStock.textContent = isOut ? (isEn ? 'Out of Stock' : 'Stok Habis') : (isEn ? `Available: ${stockText}` : `Tersedia: ${stockText}`);
    if (isOut) modalStock.style.color = '#f87171';
  }
  if (isOut) {
    const buyBtn = document.getElementById('btnOpenCheckoutFromDetail');
    if (buyBtn) {
      buyBtn.disabled = true;
      buyBtn.textContent = isEn ? 'Product Out of Stock' : 'Stok Produk Habis';
      buyBtn.style.opacity = '0.6';
      buyBtn.style.cursor = 'not-allowed';
    }
    const stickyBtn = document.getElementById('btnStickyBuy');
    if (stickyBtn) {
      stickyBtn.disabled = true;
      stickyBtn.textContent = isEn ? 'Out of Stock' : 'Stok Habis';
      stickyBtn.style.opacity = '0.6';
      stickyBtn.style.cursor = 'not-allowed';
    }
    const modalInput = document.getElementById('modalQtyInput');
    if (modalInput) modalInput.disabled = true;
    const btnMMinus = document.getElementById('btnModalQtyMinus');
    if (btnMMinus) btnMMinus.disabled = true;
    const btnMPlus = document.getElementById('btnModalQtyPlus');
    if (btnMPlus) btnMPlus.disabled = true;
  }

  // Promo Strikethrough & Badges
  const promoStrikeWrap = document.getElementById('dPromoStrikeWrap');
  const stickyStrikeWrap = document.getElementById('mStickyStrikeWrap');

  if (p.hasPromo) {
    if (promoStrikeWrap) {
      promoStrikeWrap.style.display = 'flex';
      document.getElementById('dStrikedPrice').textContent = formatIDR(p.referencePriceIDR);
      document.getElementById('dDiscountBadge').textContent = `-${p.discountPercent}% OFF`;
    }
    if (stickyStrikeWrap) {
      stickyStrikeWrap.style.display = 'flex';
      document.getElementById('mStickyStrikedPrice').textContent = formatIDR(p.referencePriceIDR);
      document.getElementById('mStickyDiscountBadge').textContent = `-${p.discountPercent}%`;
    }
    document.getElementById('dUnitPriceDisplay')?.classList.add('promo-highlight');
  } else {
    if (promoStrikeWrap) promoStrikeWrap.style.display = 'none';
    if (stickyStrikeWrap) stickyStrikeWrap.style.display = 'none';
    document.getElementById('dUnitPriceDisplay')?.classList.remove('promo-highlight');
  }

  // Description
  document.getElementById('dProdDesc').textContent = p.description || 'Lisensi akun premium langsung aktif setelah pembayaran diverifikasi.';

  // Terms Box (Default: Membeli berarti setuju)
  const termsText = p.terms || 'Membeli berarti setuju.';
  document.getElementById('dProdTerms').textContent = termsText;

  updateCalculations();
}

function getEffectiveUnitPrice(qty) {
  if (!currentProduct) return 0;
  let price = currentProduct.priceIDR;
  // Jika produk dikustomisasi oleh admin atau sedang ada promo, harga dari katalog index WAJIB digunakan (tidak boleh ditimpa tiers grosir upstream)
  if (!currentProduct.isCustomized && !currentProduct.hasPromo && currentProduct.tiers && currentProduct.tiers.length > 0) {
    for (const tier of currentProduct.tiers) {
      if (qty >= tier.minQty && (tier.maxQty === null || tier.maxQty === undefined || qty <= tier.maxQty)) {
        if (tier.priceIDR && tier.priceIDR > 0) {
          price = tier.priceIDR;
        }
        break;
      }
    }
  }
  return price;
}

function updateCalculations() {
  if (!currentProduct) return;

  const unitPrice = getEffectiveUnitPrice(currentQty);
  const subtotal = unitPrice * currentQty;

  document.getElementById('dUnitPriceDisplay').textContent = formatIDR(unitPrice);
  document.getElementById('dQtySummary').textContent = `${currentQty} item`;
  document.getElementById('dSubtotalDisplay').textContent = formatIDR(subtotal);

  const stickyPrice = document.getElementById('mStickyPrice');
  if (stickyPrice) stickyPrice.textContent = formatIDR(unitPrice);

  // Voucher discount calculation & minimum spend validation
  let finalTotal = subtotal;
  if (appliedVoucher && appliedVoucher.discountIDR > 0) {
    if (appliedVoucher.minSpendIDR > 0 && subtotal < appliedVoucher.minSpendIDR) {
      const vCode = appliedVoucher.code;
      const vMin = appliedVoucher.minSpendIDR;
      handleCancelVoucher(true);
      const feedback = document.getElementById('voucherFeedback');
      if (feedback) {
        feedback.textContent = `Voucher ${vCode} dibatalkan otomatis: minimal belanja ${formatIDR(vMin)} belum terpenuhi.`;
        feedback.style.color = '#f87171';
        feedback.style.display = 'block';
      }
      showToast(`Voucher ${vCode} dibatalkan karena kuantitas berkurang.`);
      return;
    }

    const effectiveDiscount = Math.min(appliedVoucher.discountIDR, Math.max(0, subtotal - 1000));
    finalTotal = Math.max(1000, subtotal - effectiveDiscount);

    const vRow = document.getElementById('mVoucherRow');
    if (vRow) vRow.style.display = 'flex';
    const vTag = document.getElementById('mVoucherCodeTag');
    if (vTag) vTag.textContent = appliedVoucher.code;
    const vAmount = document.getElementById('mVoucherDiscountAmount');
    if (vAmount) vAmount.textContent = `-${formatIDR(effectiveDiscount)}`;
  } else {
    const vRow = document.getElementById('mVoucherRow');
    if (vRow) vRow.style.display = 'none';
  }

  // Modal checkout updates
  const isCrypto = (selectedPaymentMethod === 'BINANCE_PAY' || selectedPaymentMethod === 'CRYPTO');
  const usdRate = appConfig.usd_rate > 0 ? appConfig.usd_rate : 18000;
  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  if (isCrypto) {
    const usdAmount = (finalTotal / usdRate).toFixed(2);
    const unitUsd = (unitPrice / usdRate).toFixed(2);
    document.getElementById('mUnitPrice').textContent = `$${unitUsd} USDT`;
    document.getElementById('mTotalPrice').textContent = `$${usdAmount} USDT`;
  } else {
    const cur = window.DigimestCurrency ? window.DigimestCurrency.getCurrency() : 'IDR';
    if (cur === 'USD') {
      const usdAmount = (finalTotal / usdRate).toFixed(2);
      const unitUsd = (unitPrice / usdRate).toFixed(2);
      document.getElementById('mUnitPrice').textContent = `$${unitUsd} (~Rp ${Number(unitPrice).toLocaleString('id-ID')})`;
      document.getElementById('mTotalPrice').textContent = `Rp ${Number(finalTotal).toLocaleString('id-ID')} (~$${usdAmount})`;
    } else {
      document.getElementById('mUnitPrice').textContent = 'Rp ' + Number(unitPrice).toLocaleString('id-ID');
      document.getElementById('mTotalPrice').textContent = 'Rp ' + Number(finalTotal).toLocaleString('id-ID');
    }
  }
  document.getElementById('mQtyDisplay').textContent = `x ${currentQty}`;

  const btn = document.getElementById('btnConfirmCheckout');
  if (btn && !btn.disabled) {
    if (selectedPaymentMethod === 'BINANCE_PAY') {
      btn.textContent = isEn ? 'Pay Now via Binance Pay ➔' : 'Bayar Sekarang via Binance Pay ➔';
    } else if (selectedPaymentMethod === 'CRYPTO' || selectedPaymentGateway === 'USDT_BEP20') {
      btn.textContent = isEn ? 'Pay Now via USDT (BEP-20) ➔' : 'Bayar Sekarang via USDT (BEP-20) ➔';
    } else {
      btn.textContent = isEn ? 'Pay Now via QRIS ➔' : 'Bayar Sekarang via QRIS ➔';
    }
  }

  const modalQtyInput = document.getElementById('modalQtyInput');
  if (modalQtyInput && parseInt(modalQtyInput.value, 10) !== currentQty) {
    modalQtyInput.value = currentQty;
  }
}

function setupPaymentSelectorListeners() {
  const optQRIS = document.getElementById('optPayQRIS');
  const optBinance = document.getElementById('optPayBinance');
  const optBep20 = document.getElementById('optPayBep20');

  optQRIS?.addEventListener('click', () => {
    selectPaymentMethod('PAYMENTG', 'QRIS', 'SHOPEEPAY');
  });

  optBinance?.addEventListener('click', () => {
    selectPaymentMethod('BINANCE', 'BINANCE_PAY', 'USDT');
  });

  optBep20?.addEventListener('click', () => {
    selectPaymentMethod('USDT_BEP20', 'CRYPTO', 'BEP20');
  });
}

function selectPaymentMethod(gateway, method, channel) {
  selectedPaymentGateway = gateway;
  selectedPaymentMethod = method;
  selectedPaymentChannel = channel;

  const optQRIS = document.getElementById('optPayQRIS');
  const optBinance = document.getElementById('optPayBinance');
  const optBep20 = document.getElementById('optPayBep20');
  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  optQRIS?.classList.remove('active');
  optBinance?.classList.remove('active');
  optBep20?.classList.remove('active');

  const badge = document.getElementById('mCurrencyBadge');
  const sumMethod = document.getElementById('mSummaryPaymentMethod');
  const rateRow = document.getElementById('mSummaryUsdRateRow');

  if (method === 'BINANCE_PAY') {
    optBinance?.classList.add('active');
    if (badge) {
      badge.textContent = 'USD / USDT';
      badge.style.background = 'rgba(245, 158, 11, 0.15)';
      badge.style.color = '#fbbf24';
    }
    if (sumMethod) {
      sumMethod.textContent = '⚡ Binance Pay (USDT)';
      sumMethod.style.color = '#fbbf24';
    }
    if (rateRow) rateRow.style.display = 'flex';
  } else if (method === 'CRYPTO' || gateway === 'USDT_BEP20') {
    optBep20?.classList.add('active');
    if (badge) {
      badge.textContent = 'USDT (BEP-20)';
      badge.style.background = 'rgba(240, 185, 11, 0.15)';
      badge.style.color = '#f0b90b';
    }
    if (sumMethod) {
      sumMethod.textContent = '🟡 USDT BEP-20 (BSC)';
      sumMethod.style.color = '#f0b90b';
    }
    if (rateRow) rateRow.style.display = 'flex';
  } else {
    optQRIS?.classList.add('active');
    if (badge) {
      badge.textContent = 'IDR (Rupiah)';
      badge.style.background = 'rgba(16, 185, 129, 0.15)';
      badge.style.color = '#10b981';
    }
    if (sumMethod) {
      sumMethod.textContent = isEn ? 'Instant QRIS (Multi-Bank)' : 'QRIS Instant Multi-Bank';
      sumMethod.style.color = '#60a5fa';
    }
    if (rateRow) rateRow.style.display = 'none';
  }

  updateCalculations();
}

function getMaxAvailableQty() {
  if (currentProduct && currentProduct.stock > 0 && !currentProduct.stockCapped) {
    return Math.min(currentProduct.stock, 100);
  }
  return 100;
}

function setupEventListeners() {
  setupPaymentSelectorListeners();
  const qtyInput = document.getElementById('detailQtyInput');
  const btnMinus = document.getElementById('btnQtyMinus');
  const btnPlus = document.getElementById('btnQtyPlus');

  btnMinus?.addEventListener('click', () => {
    if (currentQty > 1) {
      currentQty--;
      qtyInput.value = currentQty;
      updateCalculations();
    }
  });

  btnPlus?.addEventListener('click', () => {
    const maxQty = getMaxAvailableQty();
    if (currentQty < maxQty) {
      currentQty++;
      qtyInput.value = currentQty;
      updateCalculations();
    } else if (currentProduct && currentProduct.stock > 0 && currentQty >= currentProduct.stock) {
      showToast(`Batas stok produk ini tersisa ${currentProduct.stock} unit.`);
    }
  });

  qtyInput?.addEventListener('input', () => {
    const maxQty = getMaxAvailableQty();
    let val = parseInt(qtyInput.value, 10) || 1;
    if (val < 1) val = 1;
    if (val > maxQty) {
      val = maxQty;
      if (currentProduct && currentProduct.stock > 0 && maxQty === currentProduct.stock) {
        showToast(`Batas stok produk ini tersisa ${currentProduct.stock} unit.`);
      }
    }
    currentQty = val;
    qtyInput.value = val;
    updateCalculations();
  });

  // Modal checkout quantity stepper handlers
  const modalQty = document.getElementById('modalQtyInput');
  const btnModalMinus = document.getElementById('btnModalQtyMinus');
  const btnModalPlus = document.getElementById('btnModalQtyPlus');

  btnModalMinus?.addEventListener('click', () => {
    if (currentQty > 1) {
      currentQty--;
      if (qtyInput) qtyInput.value = currentQty;
      if (modalQty) modalQty.value = currentQty;
      updateCalculations();
    }
  });

  btnModalPlus?.addEventListener('click', () => {
    const maxQty = getMaxAvailableQty();
    if (currentQty < maxQty) {
      currentQty++;
      if (qtyInput) qtyInput.value = currentQty;
      if (modalQty) modalQty.value = currentQty;
      updateCalculations();
    } else if (currentProduct && currentProduct.stock > 0 && currentQty >= currentProduct.stock) {
      showToast(`Batas stok produk ini tersisa ${currentProduct.stock} unit.`);
    }
  });

  modalQty?.addEventListener('input', () => {
    const maxQty = getMaxAvailableQty();
    let val = parseInt(modalQty.value, 10) || 1;
    if (val < 1) val = 1;
    if (val > maxQty) {
      val = maxQty;
      if (currentProduct && currentProduct.stock > 0 && maxQty === currentProduct.stock) {
        showToast(`Batas stok produk ini tersisa ${currentProduct.stock} unit.`);
      }
    }
    currentQty = val;
    modalQty.value = val;
    if (qtyInput) qtyInput.value = val;
    updateCalculations();
  });

  // Voucher handlers
  document.getElementById('btnApplyVoucher')?.addEventListener('click', handleApplyVoucher);
  document.getElementById('btnCancelVoucher')?.addEventListener('click', handleCancelVoucher);
  document.getElementById('voucherInput')?.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleApplyVoucher();
    }
  });

  // Modal Checkout Trigger
  document.getElementById('btnOpenCheckoutFromDetail')?.addEventListener('click', openCheckoutModal);
  document.getElementById('btnStickyBuy')?.addEventListener('click', openCheckoutModal);
  document.getElementById('closeCheckoutBtn')?.addEventListener('click', closeCheckoutModal);

  // Submit Checkout
  document.getElementById('btnConfirmCheckout')?.addEventListener('click', submitCheckout);

  // History Drawer
  document.getElementById('btnOpenHistory')?.addEventListener('click', openHistoryModal);
  document.getElementById('closeHistoryBtn')?.addEventListener('click', closeHistoryModal);

  // Smooth scroll to customer reviews section
  document.getElementById('dRatingLink')?.addEventListener('click', (e) => {
    e.preventDefault();
    const target = document.getElementById('customerReviewsSection');
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  });

  // Close modals on background click
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) {
      closeCheckoutModal();
      closeHistoryModal();
    }
  });
}

async function handleApplyVoucher() {
  if (!currentProduct) return;
  const input = document.getElementById('voucherInput');
  const code = (input?.value || '').trim().toUpperCase();
  const feedback = document.getElementById('voucherFeedback');
  const btnApply = document.getElementById('btnApplyVoucher');

  if (!code) {
    if (feedback) {
      feedback.textContent = 'Silakan ketik kode voucher terlebih dahulu.';
      feedback.style.color = '#f87171';
      feedback.style.display = 'block';
    }
    input?.focus();
    return;
  }

  btnApply.disabled = true;
  btnApply.textContent = 'Memeriksa...';
  if (feedback) feedback.style.display = 'none';

  try {
    const validateUrl = window.digimestApi ? window.digimestApi('/api/vouchers/validate') : '/api/vouchers/validate';
    const res = await fetch(validateUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        code: code,
        product_slug: currentProduct.slug,
        quantity: currentQty
      })
    });

    const json = await res.json();
    if (json.success && json.data && json.data.valid) {
      appliedVoucher = {
        code: json.data.code,
        discountIDR: json.data.discount_idr,
        minSpendIDR: json.data.min_spend_idr || 0
      };

      if (feedback) {
        feedback.textContent = `✓ ${json.data.message || 'Voucher berhasil diterapkan!'}`;
        feedback.style.color = '#10b981';
        feedback.style.display = 'block';
      }

      input.disabled = true;
      btnApply.style.display = 'none';
      const btnCancel = document.getElementById('btnCancelVoucher');
      if (btnCancel) btnCancel.style.display = 'inline-block';
      const statusTag = document.getElementById('voucherStatusTag');
      if (statusTag) {
        statusTag.textContent = `Aktif (${json.data.code})`;
        statusTag.style.display = 'inline';
      }

      updateCalculations();
      showToast(`Voucher ${json.data.code} berhasil diterapkan! ✨`);
    } else {
      appliedVoucher = null;
      if (feedback) {
        feedback.textContent = `✕ ${json.error || 'Kode voucher tidak valid atau kuota telah habis'}`;
        feedback.style.color = '#f87171';
        feedback.style.display = 'block';
      }
      updateCalculations();
    }
  } catch (err) {
    if (feedback) {
      feedback.textContent = 'Gagal menghubungi server validasi voucher.';
      feedback.style.color = '#f87171';
      feedback.style.display = 'block';
    }
  } finally {
    btnApply.disabled = false;
    btnApply.textContent = 'Terapkan';
  }
}

function handleCancelVoucher(keepFeedbackMsg = false) {
  appliedVoucher = null;
  const input = document.getElementById('voucherInput');
  if (input) {
    input.disabled = false;
    input.value = '';
    if (!keepFeedbackMsg) input.focus();
  }
  const btnApply = document.getElementById('btnApplyVoucher');
  if (btnApply) {
    btnApply.disabled = false;
    btnApply.style.display = 'inline-block';
  }
  const btnCancel = document.getElementById('btnCancelVoucher');
  if (btnCancel) btnCancel.style.display = 'none';
  const feedback = document.getElementById('voucherFeedback');
  if (feedback && !keepFeedbackMsg) feedback.style.display = 'none';
  const statusTag = document.getElementById('voucherStatusTag');
  if (statusTag) statusTag.style.display = 'none';

  updateCalculations();
}

function openCheckoutModal() {
  if (!currentProduct) return;
  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  if (currentProduct.stock <= 0 || currentProduct.isForceOutOfStock) {
    alert(isEn ? 'Sorry, this product is currently out of stock.' : 'Mohon maaf, stok produk ini sedang habis atau dinonaktifkan sementara.');
    return;
  }
  const modal = document.getElementById('checkoutModal');
  document.getElementById('mProdName').textContent = currentProduct.name;
  document.getElementById('mProdType').textContent = currentProduct.type || 'Digital License';
  document.getElementById('mProdLogo').src = currentProduct.logo || 'https://via.placeholder.com/40';

  const emailInput = document.getElementById('emailInput');
  const savedEmail = localStorage.getItem('digimest_saved_email') || localStorage.getItem('digihub_saved_email') || localStorage.getItem('storeku_saved_email');
  if (emailInput && savedEmail) {
    emailInput.value = savedEmail;
  }

  updateCalculations();
  modal?.classList.add('open');

  if (emailInput && !emailInput.value) {
    setTimeout(() => emailInput.focus(), 150);
  }
}

function closeCheckoutModal() {
  document.getElementById('checkoutModal')?.classList.remove('open');
}

async function submitCheckout() {
  if (!currentProduct) return;
  const isEn = (document.documentElement.lang === 'en') || window.location.pathname.startsWith('/en');

  const btn = document.getElementById('btnConfirmCheckout');
  const email = (document.getElementById('emailInput')?.value || '').trim();
  const contact = (document.getElementById('contactInput')?.value || '').trim();

  if (!email || !email.includes('@') || !email.includes('.')) {
    alert(isEn ? 'Please enter a valid email address for digital license delivery!' : 'Silakan masukkan alamat email yang valid untuk pengiriman kode klaim!');
    document.getElementById('emailInput')?.focus();
    return;
  }

  const isBinance = (selectedPaymentMethod === 'BINANCE_PAY');
  const isBep20 = (selectedPaymentGateway === 'USDT_BEP20' || selectedPaymentMethod === 'CRYPTO');
  btn.disabled = true;
  if (isBinance) {
    btn.innerText = isEn ? 'Preparing Binance Pay...' : 'Menyiapkan Binance Pay (USDT)...';
  } else if (isBep20) {
    btn.innerText = isEn ? 'Preparing USDT (BEP-20)...' : 'Menyiapkan USDT (BEP-20)...';
  } else {
    btn.innerText = isEn ? 'Preparing Instant QRIS...' : 'Menyiapkan QRIS ShopeePay...';
  }

  const defaultBtnText = isBinance 
    ? (isEn ? 'Pay Now via Binance Pay ➔' : 'Bayar Sekarang via Binance Pay ➔')
    : (isBep20
        ? (isEn ? 'Pay Now via USDT (BEP-20) ➔' : 'Bayar Sekarang via USDT (BEP-20) ➔')
        : (isEn ? 'Pay Now via QRIS ➔' : 'Bayar Sekarang via QRIS ➔'));

  try {
    const checkoutUrl = window.digimestApi ? window.digimestApi('/api/checkout') : '/api/checkout';
    const res = await fetch(checkoutUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        product_slug: currentProduct.slug,
        quantity: currentQty,
        customer_email: email,
        customer_contact: contact,
        voucher_code: appliedVoucher ? appliedVoucher.code : '',
        payment_gateway: selectedPaymentGateway,
        payment_method: selectedPaymentMethod,
        payment_channel: selectedPaymentChannel
      })
    });

    const data = await res.json();
    if (!data.success) {
      alert(isEn ? ('Failed to create order: ' + (data.error || 'An error occurred')) : ('Gagal membuat pesanan: ' + (data.error || 'Terjadi kesalahan')));
      btn.disabled = false;
      btn.innerText = defaultBtnText;
      return;
    }

    const order = data.data;

    // SAVE EMAIL & ORDER TO LOCAL STORAGE!
    localStorage.setItem('digimest_saved_email', email); localStorage.setItem('digihub_saved_email', email);
    localStorage.setItem('storeku_saved_email', email);
    if ((window.DigimestOrders || window.DigihubOrders)) {
      (window.DigimestOrders || window.DigihubOrders).saveOrder({
        invoice_number: order.invoice_number,
        claim_code: order.claim_code,
        product_name: order.product_name,
        quantity: order.quantity,
        total_amount: order.total_amount,
        crypto_amount_str: order.crypto_amount_str || '',
        crypto_currency: order.crypto_currency || '',
        payment_gateway: order.payment_gateway || selectedPaymentGateway,
        payment_method: order.payment_method || selectedPaymentMethod,
        customer_email: email,
        payment_status: 'PENDING',
        expires_at: order.expires_at,
        created_at: new Date().toISOString()
      });
    }

    // Redirect to Payment Screen (Clean URL, matching current language)
    const paymentPath = isEn ? '/en/payment' : '/payment';
    window.location.href = `${paymentPath}?inv=${encodeURIComponent(order.invoice_number)}`;

  } catch (err) {
    console.error('Checkout error:', err);
    alert(isEn ? 'Network error occurred during checkout.' : 'Terjadi kesalahan jaringan saat checkout.');
    btn.disabled = false;
    btn.innerText = defaultBtnText;
  }
}

// LocalStorage Order History (Delegates to unified StoreKuOrders)
function saveHistoryItem(item) {
  if (window.StoreKuOrders) {
    StoreKuOrders.saveOrder(item);
  }
}

function openHistoryModal() {
  if (window.StoreKuOrders) {
    StoreKuOrders.renderHistoryList('historyListContainer');
  }
  document.getElementById('historyModal')?.classList.add('open');
}

function closeHistoryModal() {
  document.getElementById('historyModal')?.classList.remove('open');
}

// Reactive Currency Switcher Listener for Product Detail
window.addEventListener('currencychange', () => {
  if (currentProduct) {
    const promoStrikeWrap = document.getElementById('dPromoStrikeWrap');
    const stickyStrikeWrap = document.getElementById('mStickyStrikeWrap');
    if (currentProduct.hasPromo) {
      if (promoStrikeWrap) {
        const dStrike = document.getElementById('dStrikedPrice');
        if (dStrike) dStrike.textContent = formatCurrency(currentProduct.referencePriceIDR);
      }
      if (stickyStrikeWrap) {
        const mStrike = document.getElementById('mStickyStrikedPrice');
        if (mStrike) mStrike.textContent = formatCurrency(currentProduct.referencePriceIDR);
      }
    }
    updateCalculations();
  }
});
