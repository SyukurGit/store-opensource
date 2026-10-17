// Digimest Frontend Catalog & Checkout Engine

let allProducts = [];
let selectedProduct = null;

// Format Currency
function formatCurrency(amountIDR, amountUSD = 0) {
  if (window.DigimestCurrency && typeof window.DigimestCurrency.format === 'function') {
    return window.DigimestCurrency.format(amountIDR, amountUSD);
  }
  return 'Rp ' + Number(amountIDR || 0).toLocaleString('id-ID');
}

function formatIDR(num) {
  return formatCurrency(num, 0);
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
  const handler = window.DigimestOrders || window.DigihubOrders || window.StoreKuOrders;
  return handler ? handler.getOrders() : [];
}

function saveHistoryItem(item) {
  const handler = window.DigimestOrders || window.DigihubOrders || window.StoreKuOrders;
  if (handler) {
    handler.saveOrder(item);
  }
}

function updateHistoryBadge() {
  const handler = window.DigimestOrders || window.DigihubOrders || window.StoreKuOrders;
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
  const isEn = document.documentElement.lang === 'en' || window.location.pathname.startsWith('/en');
  const prodBaseUrl = isEn ? '/en/product?slug=' : '/product?slug=';

  grid.innerHTML = products.map(p => {
    const isOut = p.stock <= 0 || !!p.isForceOutOfStock;
    const stockDisplay = isOut
      ? (isEn ? 'Out of Stock' : 'Habis')
      : (p.stockCapped ? (isEn ? '>1000 In Stock' : '>1000 Stok') : (isEn ? `${p.stock} In Stock` : `${p.stock} Stok`));
    const logoSrc = p.logo || 'https://via.placeholder.com/46?text=DC';
    const prodLink = prodBaseUrl + encodeURIComponent(p.slug);

    return `
      <div class="product-card ${isOut ? 'out-of-stock' : ''}" onclick="window.location.href='${prodLink}'" style="cursor: pointer;">
        <div class="prod-card-body">
          <div class="prod-header">
            <img src="${logoSrc}" alt="${escapeHtml(p.name)}" class="prod-logo" onerror="this.src='https://via.placeholder.com/46?text=DC'">
            <div class="prod-title-wrap">
              <div class="prod-header-meta">
                <span class="prod-category">${escapeHtml(p.type || 'Digital License')}</span>
                <span class="prod-sold-badge">${p.salesCount || 10} ${isEn ? 'sold' : 'terjual'}</span>
              </div>
              <h3 class="prod-title" title="${escapeHtml(p.name)}">${escapeHtml(p.name)}</h3>
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

          <div class="prod-desc">${escapeHtml(p.description || (isEn ? 'Instant premium digital license delivered.' : 'Lisensi akun premium langsung aktif.'))}</div>
        </div>

        <div class="prod-footer">
          <div class="price-wrap">
            ${p.hasPromo ? `
              <div class="price-strike-wrap">
                <span class="price-original-striked">${formatCurrency(p.referencePriceIDR)}</span>
                <span class="badge-discount-mini">-${p.discountPercent}%</span>
              </div>
              <span class="price-amount promo-highlight">${formatCurrency(p.priceIDR)}</span>
            ` : `
              <span class="price-label">${isEn ? 'Starting From' : 'Mulai Dari'}</span>
              <span class="price-amount">${formatCurrency(p.priceIDR)}</span>
            `}
          </div>
          <button class="btn-buy btn-buy-catalog ${isOut ? 'btn-buy-out' : ''}" onclick="event.stopPropagation(); window.location.href='${prodLink}'">
            ${isOut ? (isEn ? 'Sold Out' : 'Habis') : (isEn ? 'Buy ➔' : 'Beli ➔')}
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

// History Modal Render (Uses unified DigihubOrders)
function openHistoryModal() {
  const orderHandler = window.DigimestOrders || window.DigihubOrders || window.StoreKuOrders;
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

// ==========================================================================
// HERO CAROUSEL CONTROLLER (60FPS HARDWARE ACCELERATED & TOUCH SWIPE)
// ==========================================================================
// ==========================================================================
// HERO CAROUSEL CONTROLLER (60FPS HARDWARE ACCELERATED & TOUCH SWIPE)
// ==========================================================================
function createCarouselController({ containerId, trackId, prevBtnId, nextBtnId, dotsSelector, progressId, duration = 3500 }) {
  const container = document.getElementById(containerId);
  const track = document.getElementById(trackId);
  const prevBtn = document.getElementById(prevBtnId);
  const nextBtn = document.getElementById(nextBtnId);
  const progressBar = document.getElementById(progressId);

  if (!container || !track) return null;

  const slides = container.querySelectorAll('.carousel-slide');
  const dots = document.querySelectorAll(dotsSelector);

  if (slides.length <= 1) return null;

  const totalSlides = slides.length;
  let currentIndex = 0;
  const slideDuration = duration;
  let slideTimer = null;
  let progressTimer = null;
  let progressStart = Date.now();
  let isPaused = false;

  function updateSlide(index, animate = true) {
    if (index < 0) {
      currentIndex = totalSlides - 1;
    } else if (index >= totalSlides) {
      currentIndex = 0;
    } else {
      currentIndex = index;
    }

    if (!animate) {
      track.style.transition = 'none';
    } else {
      track.style.transition = 'transform 0.55s cubic-bezier(0.2, 0.9, 0.3, 1)';
    }

    track.style.transform = `translate3d(-${currentIndex * 100}%, 0, 0)`;

    // Update active classes for slides & dots
    slides.forEach((slide, i) => {
      slide.classList.toggle('active', i === currentIndex);
    });

    dots.forEach((dot, i) => {
      const isActive = i === currentIndex;
      dot.classList.toggle('active', isActive);
      dot.setAttribute('aria-selected', isActive ? 'true' : 'false');
    });

    resetProgress();
  }

  function startAutoplay() {
    clearTimeout(slideTimer);
    clearInterval(progressTimer);
    if (isPaused) return;

    progressStart = Date.now();
    if (progressBar) progressBar.style.width = '0%';

    progressTimer = setInterval(() => {
      if (isPaused) return;
      const elapsed = Date.now() - progressStart;
      const pct = Math.min(100, (elapsed / slideDuration) * 100);
      if (progressBar) progressBar.style.width = pct + '%';
    }, 40);

    slideTimer = setTimeout(() => {
      if (!isPaused) {
        updateSlide(currentIndex + 1);
      }
    }, slideDuration);
  }

  function resetProgress() {
    clearTimeout(slideTimer);
    clearInterval(progressTimer);
    startAutoplay();
  }

  function pause() {
    isPaused = true;
    clearTimeout(slideTimer);
    clearInterval(progressTimer);
  }

  function resume() {
    if (!isPaused) return;
    isPaused = false;
    startAutoplay();
  }

  // Button Listeners
  prevBtn?.addEventListener('click', (e) => {
    e.stopPropagation();
    updateSlide(currentIndex - 1);
  });

  nextBtn?.addEventListener('click', (e) => {
    e.stopPropagation();
    updateSlide(currentIndex + 1);
  });

  // Dots click
  dots.forEach(dot => {
    dot.addEventListener('click', (e) => {
      e.stopPropagation();
      const idx = parseInt(e.currentTarget.dataset.index, 10);
      if (!isNaN(idx)) {
        updateSlide(idx);
      }
    });
  });

  // Pause on hover
  container.addEventListener('mouseenter', pause);
  container.addEventListener('mouseleave', resume);

  // Touch Swipe Gesture
  let touchStartX = 0;
  let touchStartY = 0;
  let touchEndX = 0;
  let touchEndY = 0;
  let isSwiping = false;

  container.addEventListener('touchstart', (e) => {
    pause();
    const touch = e.touches[0];
    touchStartX = touch.clientX;
    touchStartY = touch.clientY;
    touchEndX = touch.clientX;
    touchEndY = touch.clientY;
    isSwiping = true;
  }, { passive: true });

  container.addEventListener('touchmove', (e) => {
    if (!isSwiping) return;
    const touch = e.touches[0];
    touchEndX = touch.clientX;
    touchEndY = touch.clientY;
  }, { passive: true });

  container.addEventListener('touchend', () => {
    if (!isSwiping) return;
    isSwiping = false;
    const diffX = touchEndX - touchStartX;
    const diffY = touchEndY - touchStartY;

    if (Math.abs(diffX) > Math.abs(diffY) && Math.abs(diffX) > 40) {
      if (diffX < 0) {
        updateSlide(currentIndex + 1);
      } else {
        updateSlide(currentIndex - 1);
      }
    }
    resume();
  });

  // Page Visibility Change
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      pause();
    } else {
      resume();
    }
  });

  // Keyboard navigation
  container.setAttribute('tabindex', '0');
  container.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowLeft') {
      updateSlide(currentIndex - 1);
    } else if (e.key === 'ArrowRight') {
      updateSlide(currentIndex + 1);
    }
  });

  // Init First Slide & Run
  updateSlide(0, false);
  startAutoplay();

  return { updateSlide, pause, resume };
}

function initHeroCarousels() {
  // Desktop Slider (Fast 3200ms cycle between banner1 and banner2)
  createCarouselController({
    containerId: 'heroCarouselDesktop',
    trackId: 'carouselTrackDesktop',
    prevBtnId: 'carouselPrevBtnDesktop',
    nextBtnId: 'carouselNextBtnDesktop',
    dotsSelector: '#carouselPaginationDesktop .carousel-dot',
    progressId: 'carouselProgressBarDesktop',
    duration: 3200
  });

  // Mobile Slider (Smooth 4500ms cycle across all 3 banners)
  createCarouselController({
    containerId: 'heroCarouselMobile',
    trackId: 'carouselTrackMobile',
    prevBtnId: 'carouselPrevBtnMobile',
    nextBtnId: 'carouselNextBtnMobile',
    dotsSelector: '#carouselPaginationMobile .carousel-dot',
    progressId: 'carouselProgressBarMobile',
    duration: 4500
  });
}

// ==========================================================================
// ENHANCED PROMINENT SEARCH CONTROLLER (DESKTOP & MOBILE)
// ==========================================================================
function initEnhancedSearch() {
  const searchInput = document.getElementById('searchInput');
  const searchClearBtn = document.getElementById('searchClearBtn');
  const searchSubmitBtn = document.getElementById('searchSubmitBtn');

  if (!searchInput) return;

  function handleSearchInput() {
    const val = searchInput.value.trim();
    if (searchClearBtn) {
      searchClearBtn.style.display = val.length > 0 ? 'flex' : 'none';
    }
    applyFilterAndSearch();
  }

  function handleSearchSubmit() {
    applyFilterAndSearch();
    // Scroll smoothly to products catalog section if user presses search button or enter
    const target = document.getElementById('productsGrid') || document.getElementById('loadingCatalog');
    if (target) {
      const topOffset = target.getBoundingClientRect().top + window.pageYOffset - 100;
      window.scrollTo({ top: topOffset, behavior: 'smooth' });
    }
  }

  searchInput.addEventListener('input', handleSearchInput);

  searchInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSearchSubmit();
    }
  });

  searchSubmitBtn?.addEventListener('click', (e) => {
    e.preventDefault();
    handleSearchSubmit();
  });

  searchClearBtn?.addEventListener('click', (e) => {
    e.preventDefault();
    searchInput.value = '';
    searchClearBtn.style.display = 'none';
    searchInput.focus();
    applyFilterAndSearch();
  });
}

// Lifecycle Init
document.addEventListener('DOMContentLoaded', async () => {
  updateHistoryBadge();
  initHeroCarousels();
  initEnhancedSearch();

  // Load products
  try {
    const productsUrl = window.digimestApi ? window.digimestApi('/api/products') : '/api/products';
    const res = await fetch(productsUrl);
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

  // Filter tag buttons
  document.querySelectorAll('.tag-btn').forEach(btn => {
    btn.addEventListener('click', (e) => {
      document.querySelectorAll('.tag-btn').forEach(b => b.classList.remove('active'));
      e.target.classList.add('active');
      applyFilterAndSearch();
    });
  });

  document.getElementById('openHistoryBtn')?.addEventListener('click', openHistoryModal);
  document.getElementById('closeHistoryBtn')?.addEventListener('click', closeHistoryModal);

  // Close modals on clicking overlay outside card
  window.addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) {
      closeHistoryModal();
    }
  });

  // Reactive Currency Switcher Listener
  window.addEventListener('currencychange', () => {
    if (allProducts && allProducts.length > 0) {
      applyFilterAndSearch();
    }
  });
});
